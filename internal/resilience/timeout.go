package resilience

import (
	"context"
	"errors"
	"fmt"
	"time"

	"raioz/internal/logging"
)

// TimeoutConfig configures timeout behavior
type TimeoutConfig struct {
	// DefaultTimeout is the default timeout for operations
	DefaultTimeout time.Duration
	// OperationTimeouts maps operation names to specific timeouts
	OperationTimeouts map[string]time.Duration
}

// DefaultTimeoutConfig returns a default timeout configuration
func DefaultTimeoutConfig() TimeoutConfig {
	return TimeoutConfig{
		DefaultTimeout: 5 * time.Minute,
		OperationTimeouts: map[string]time.Duration{
			"git.clone":      10 * time.Minute,
			"git.pull":       5 * time.Minute,
			"git.checkout":   2 * time.Minute,
			"docker.pull":    15 * time.Minute,
			"docker.up":      5 * time.Minute,
			"docker.down":    2 * time.Minute,
			"docker.inspect": 30 * time.Second,
			"docker.status":  30 * time.Second,
			"network.check":  3 * time.Second,
		},
	}
}

// GetTimeout returns the timeout for a specific operation
func (tc TimeoutConfig) GetTimeout(operation string) time.Duration {
	if timeout, ok := tc.OperationTimeouts[operation]; ok {
		return timeout
	}
	return tc.DefaultTimeout
}

// WithTimeout creates a context with timeout and proper error handling
func WithTimeout(
	ctx context.Context, timeout time.Duration, operation string,
) (context.Context, context.CancelFunc, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	// Check if context is already cancelled
	select {
	case <-ctx.Done():
		return nil, nil, fmt.Errorf("context already cancelled: %w", ctx.Err())
	default:
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	return timeoutCtx, cancel, nil
}

// ExecuteWithTimeout executes a function with timeout and proper error handling
func ExecuteWithTimeout(
	ctx context.Context, timeout time.Duration,
	operation string, fn func(context.Context) error,
) error {
	timeoutCtx, cancel, err := WithTimeout(ctx, timeout, operation)
	if err != nil {
		return err
	}
	defer cancel()

	// Execute the operation
	done := make(chan error, 1)
	go func() {
		done <- fn(timeoutCtx)
	}()

	select {
	case err := <-done:
		// Operation completed
		if err != nil {
			// Check if it's a timeout error
			if errors.Is(timeoutCtx.Err(), context.DeadlineExceeded) {
				logging.Error("Operation timed out",
					"operation", operation,
					"timeout", timeout,
				)
				return fmt.Errorf("%s timed out after %v", operation, timeout)
			}
			return err
		}
		return nil
	case <-timeoutCtx.Done():
		// Timeout occurred
		if errors.Is(timeoutCtx.Err(), context.DeadlineExceeded) {
			logging.Error("Operation timed out",
				"operation", operation,
				"timeout", timeout,
			)
			return fmt.Errorf("%s timed out after %v", operation, timeout)
		}
		// Context was cancelled
		logging.Warn("Operation cancelled",
			"operation", operation,
			"error", timeoutCtx.Err().Error(),
		)
		return fmt.Errorf("%s cancelled: %w", operation, timeoutCtx.Err())
	}
}

// HandleTimeoutError handles timeout errors with proper context
func HandleTimeoutError(ctx context.Context, err error, operation string, timeout time.Duration) error {
	if err == nil {
		return nil
	}

	// Check if context was cancelled
	if ctx != nil {
		select {
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.Canceled) {
				return fmt.Errorf("%s was cancelled: %w", operation, ctx.Err())
			}
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return fmt.Errorf("%s timed out after %v", operation, timeout)
			}
		default:
		}
	}

	// Check if error is a timeout error
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("%s timed out after %v", operation, timeout)
	}

	return fmt.Errorf("%s failed: %w", operation, err)
}

// IsTimeoutError checks if an error is a timeout error
func IsTimeoutError(ctx context.Context, err error) bool {
	if err == nil {
		return false
	}

	if ctx != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return true
		}
	}

	return errors.Is(err, context.DeadlineExceeded)
}

// IsCancelledError checks if an error is a cancellation error
func IsCancelledError(ctx context.Context, err error) bool {
	if err == nil {
		return false
	}

	if ctx != nil {
		if errors.Is(ctx.Err(), context.Canceled) {
			return true
		}
	}

	return errors.Is(err, context.Canceled)
}
