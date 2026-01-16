package resilience

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"raioz/internal/logging"
)

// RetryConfig configures retry behavior
type RetryConfig struct {
	MaxAttempts      int
	InitialDelay     time.Duration
	MaxDelay         time.Duration
	BackoffMultiplier float64
	RetryableErrors  []error // Errors that should trigger retry
}

// DefaultRetryConfig returns a default retry configuration
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts:       3,
		InitialDelay:     1 * time.Second,
		MaxDelay:         30 * time.Second,
		BackoffMultiplier: 2.0,
		RetryableErrors:  []error{},
	}
}

// NetworkRetryConfig returns a retry configuration for network operations
func NetworkRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts:       5,
		InitialDelay:     2 * time.Second,
		MaxDelay:         60 * time.Second,
		BackoffMultiplier: 2.0,
		RetryableErrors:  []error{},
	}
}

// GitRetryConfig returns a retry configuration for Git operations
func GitRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts:       3,
		InitialDelay:     2 * time.Second,
		MaxDelay:         30 * time.Second,
		BackoffMultiplier: 2.0,
		RetryableErrors:  []error{},
	}
}

// DockerRetryConfig returns a retry configuration for Docker operations
func DockerRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts:       3,
		InitialDelay:     1 * time.Second,
		MaxDelay:         30 * time.Second,
		BackoffMultiplier: 2.0,
		RetryableErrors:  []error{},
	}
}

// IsRetryableError checks if an error is retryable
func IsRetryableError(err error, retryableErrors []error) bool {
	if err == nil {
		return false
	}

	// Check if error is context timeout (retryable for network operations)
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	// Check if error is context canceled (not retryable)
	if errors.Is(err, context.Canceled) {
		return false
	}

	// Check against list of retryable errors
	for _, retryableErr := range retryableErrors {
		if errors.Is(err, retryableErr) {
			return true
		}
	}

	// Check error message for transient error patterns
	errMsg := err.Error()
	transientPatterns := []string{
		"connection refused",
		"connection reset",
		"timeout",
		"temporary failure",
		"network is unreachable",
		"no route to host",
		"i/o timeout",
		"EOF",
		"broken pipe",
		"connection closed",
		"temporary",
		"retry",
		"rate limit",
		"too many requests",
		"service unavailable",
		"bad gateway",
		"gateway timeout",
	}

	for _, pattern := range transientPatterns {
		if contains(errMsg, pattern) {
			return true
		}
	}

	return false
}

// contains checks if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// Retry executes a function with retry logic
func Retry(ctx context.Context, config RetryConfig, operation string, fn func() error) error {
	var lastErr error
	delay := config.InitialDelay

	for attempt := 1; attempt <= config.MaxAttempts; attempt++ {
		// Check if context is cancelled
		if ctx != nil {
			select {
			case <-ctx.Done():
				return fmt.Errorf("operation cancelled: %w", ctx.Err())
			default:
			}
		}

		// Execute the operation
		err := fn()

		if err == nil {
			// Success
			if attempt > 1 {
				logging.Info("Operation succeeded after retry",
					"operation", operation,
					"attempt", attempt,
					"total_attempts", config.MaxAttempts,
				)
			}
			return nil
		}

		lastErr = err

		// Check if error is retryable
		if !IsRetryableError(err, config.RetryableErrors) {
			logging.Debug("Error is not retryable, aborting",
				"operation", operation,
				"attempt", attempt,
				"error", err.Error(),
			)
			return fmt.Errorf("%s failed (non-retryable): %w", operation, err)
		}

		// If this was the last attempt, return the error
		if attempt >= config.MaxAttempts {
			logging.Warn("Operation failed after all retry attempts",
				"operation", operation,
				"attempt", attempt,
				"total_attempts", config.MaxAttempts,
				"error", err.Error(),
			)
			return fmt.Errorf("%s failed after %d attempts: %w", operation, config.MaxAttempts, err)
		}

		// Log retry attempt
		logging.Warn("Operation failed, retrying",
			"operation", operation,
			"attempt", attempt,
			"next_attempt", attempt+1,
			"delay_ms", delay.Milliseconds(),
			"error", err.Error(),
		)

		// Wait before retrying
		if ctx != nil {
			select {
			case <-ctx.Done():
				return fmt.Errorf("operation cancelled during retry: %w", ctx.Err())
			case <-time.After(delay):
			}
		} else {
			time.Sleep(delay)
		}

		// Calculate next delay with exponential backoff
		delay = time.Duration(float64(delay) * config.BackoffMultiplier)
		if delay > config.MaxDelay {
			delay = config.MaxDelay
		}
	}

	return fmt.Errorf("%s failed after %d attempts: %w", operation, config.MaxAttempts, lastErr)
}

// RetryWithContext executes a function with retry logic and context support
func RetryWithContext(ctx context.Context, config RetryConfig, operation string, fn func(context.Context) error) error {
	var lastErr error
	delay := config.InitialDelay

	for attempt := 1; attempt <= config.MaxAttempts; attempt++ {
		// Check if context is cancelled
		select {
		case <-ctx.Done():
			return fmt.Errorf("operation cancelled: %w", ctx.Err())
		default:
		}

		// Execute the operation with context
		err := fn(ctx)

		if err == nil {
			// Success
			if attempt > 1 {
				logging.Info("Operation succeeded after retry",
					"operation", operation,
					"attempt", attempt,
					"total_attempts", config.MaxAttempts,
				)
			}
			return nil
		}

		lastErr = err

		// Check if error is retryable
		if !IsRetryableError(err, config.RetryableErrors) {
			logging.Debug("Error is not retryable, aborting",
				"operation", operation,
				"attempt", attempt,
				"error", err.Error(),
			)
			return fmt.Errorf("%s failed (non-retryable): %w", operation, err)
		}

		// If this was the last attempt, return the error
		if attempt >= config.MaxAttempts {
			logging.Warn("Operation failed after all retry attempts",
				"operation", operation,
				"attempt", attempt,
				"total_attempts", config.MaxAttempts,
				"error", err.Error(),
			)
			return fmt.Errorf("%s failed after %d attempts: %w", operation, config.MaxAttempts, err)
		}

		// Log retry attempt
		logging.Warn("Operation failed, retrying",
			"operation", operation,
			"attempt", attempt,
			"next_attempt", attempt+1,
			"delay_ms", delay.Milliseconds(),
			"error", err.Error(),
		)

		// Wait before retrying
		select {
		case <-ctx.Done():
			return fmt.Errorf("operation cancelled during retry: %w", ctx.Err())
		case <-time.After(delay):
		}

		// Calculate next delay with exponential backoff
		delay = time.Duration(float64(delay) * config.BackoffMultiplier)
		if delay > config.MaxDelay {
			delay = config.MaxDelay
		}
	}

	return fmt.Errorf("%s failed after %d attempts: %w", operation, config.MaxAttempts, lastErr)
}
