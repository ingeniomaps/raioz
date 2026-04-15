package resilience

import (
	"context"
	"fmt"
	"sync"
	"time"

	"raioz/internal/logging"
)

// CircuitState represents the state of a circuit breaker
type CircuitState int

const (
	// CircuitClosed means the circuit is closed and operations are allowed
	CircuitClosed CircuitState = iota
	// CircuitOpen means the circuit is open and operations are blocked
	CircuitOpen
	// CircuitHalfOpen means the circuit is half-open and testing if operations work
	CircuitHalfOpen
)

// String returns the string representation of the circuit state
func (s CircuitState) String() string {
	switch s {
	case CircuitClosed:
		return "closed"
	case CircuitOpen:
		return "open"
	case CircuitHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// CircuitBreakerConfig configures circuit breaker behavior
type CircuitBreakerConfig struct {
	// FailureThreshold is the number of failures before opening the circuit
	FailureThreshold int
	// SuccessThreshold is the number of successes needed to close the circuit from half-open
	SuccessThreshold int
	// Timeout is how long the circuit stays open before transitioning to half-open
	Timeout time.Duration
	// Name is the name of the circuit breaker (for logging)
	Name string
}

// DefaultCircuitBreakerConfig returns a default circuit breaker configuration
func DefaultCircuitBreakerConfig(name string) CircuitBreakerConfig {
	return CircuitBreakerConfig{
		FailureThreshold: 5,
		SuccessThreshold: 2,
		Timeout:          60 * time.Second,
		Name:             name,
	}
}

// CircuitBreaker implements a circuit breaker pattern
type CircuitBreaker struct {
	config      CircuitBreakerConfig
	state       CircuitState
	failures    int
	successes   int
	lastFailure time.Time
	mu          sync.RWMutex
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(config CircuitBreakerConfig) *CircuitBreaker {
	return &CircuitBreaker{
		config:      config,
		state:       CircuitClosed,
		failures:    0,
		successes:   0,
		lastFailure: time.Time{},
	}
}

// Execute executes a function through the circuit breaker
func (cb *CircuitBreaker) Execute(ctx context.Context, operation string, fn func() error) error {
	// Check circuit state
	cb.mu.Lock()

	// Transition from open to half-open if timeout has passed
	if cb.state == CircuitOpen {
		if time.Since(cb.lastFailure) >= cb.config.Timeout {
			cb.state = CircuitHalfOpen
			cb.successes = 0
			logging.Info("Circuit breaker transitioning to half-open",
				"circuit", cb.config.Name,
				"operation", operation,
			)
		} else {
			cb.mu.Unlock()
			return fmt.Errorf("circuit breaker is open for %s (last failure: %v ago, timeout: %v)",
				cb.config.Name,
				time.Since(cb.lastFailure),
				cb.config.Timeout,
			)
		}
	}
	cb.mu.Unlock()

	// Execute the operation
	err := fn()

	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		// Operation failed
		cb.failures++
		cb.lastFailure = time.Now()

		if cb.state == CircuitHalfOpen {
			// Half-open: any failure goes back to open
			cb.state = CircuitOpen
			cb.successes = 0
			logging.Warn("Circuit breaker opened after failure in half-open state",
				"circuit", cb.config.Name,
				"operation", operation,
				"error", err.Error(),
			)
		} else if cb.failures >= cb.config.FailureThreshold {
			// Closed: too many failures, open the circuit
			cb.state = CircuitOpen
			logging.Warn("Circuit breaker opened due to failure threshold",
				"circuit", cb.config.Name,
				"operation", operation,
				"failures", cb.failures,
				"threshold", cb.config.FailureThreshold,
				"error", err.Error(),
			)
		}

		return err
	}

	// Operation succeeded
	cb.failures = 0

	if cb.state == CircuitHalfOpen {
		cb.successes++
		if cb.successes >= cb.config.SuccessThreshold {
			// Half-open: enough successes, close the circuit
			cb.state = CircuitClosed
			cb.successes = 0
			logging.Info("Circuit breaker closed after successful operations",
				"circuit", cb.config.Name,
				"operation", operation,
				"successes", cb.successes,
			)
		}
	}

	return nil
}

// ExecuteWithContext executes a function through the circuit breaker with context support
func (cb *CircuitBreaker) ExecuteWithContext(
	ctx context.Context, operation string,
	fn func(context.Context) error,
) error {
	// Check circuit state
	cb.mu.Lock()

	// Transition from open to half-open if timeout has passed
	if cb.state == CircuitOpen {
		if time.Since(cb.lastFailure) >= cb.config.Timeout {
			cb.state = CircuitHalfOpen
			cb.successes = 0
			logging.Info("Circuit breaker transitioning to half-open",
				"circuit", cb.config.Name,
				"operation", operation,
			)
		} else {
			cb.mu.Unlock()
			return fmt.Errorf("circuit breaker is open for %s (last failure: %v ago, timeout: %v)",
				cb.config.Name,
				time.Since(cb.lastFailure),
				cb.config.Timeout,
			)
		}
	}
	cb.mu.Unlock()

	// Execute the operation
	err := fn(ctx)

	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		// Operation failed
		cb.failures++
		cb.lastFailure = time.Now()

		if cb.state == CircuitHalfOpen {
			// Half-open: any failure goes back to open
			cb.state = CircuitOpen
			cb.successes = 0
			logging.Warn("Circuit breaker opened after failure in half-open state",
				"circuit", cb.config.Name,
				"operation", operation,
				"error", err.Error(),
			)
		} else if cb.failures >= cb.config.FailureThreshold {
			// Closed: too many failures, open the circuit
			cb.state = CircuitOpen
			logging.Warn("Circuit breaker opened due to failure threshold",
				"circuit", cb.config.Name,
				"operation", operation,
				"failures", cb.failures,
				"threshold", cb.config.FailureThreshold,
				"error", err.Error(),
			)
		}

		return err
	}

	// Operation succeeded
	cb.failures = 0

	if cb.state == CircuitHalfOpen {
		cb.successes++
		if cb.successes >= cb.config.SuccessThreshold {
			// Half-open: enough successes, close the circuit
			cb.state = CircuitClosed
			cb.successes = 0
			logging.Info("Circuit breaker closed after successful operations",
				"circuit", cb.config.Name,
				"operation", operation,
				"successes", cb.successes,
			)
		}
	}

	return nil
}

// GetState returns the current state of the circuit breaker
func (cb *CircuitBreaker) GetState() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// Reset resets the circuit breaker to closed state
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.state = CircuitClosed
	cb.failures = 0
	cb.successes = 0
	cb.lastFailure = time.Time{}
	logging.Info("Circuit breaker reset",
		"circuit", cb.config.Name,
	)
}

// IsOpen returns true if the circuit breaker is open
func (cb *CircuitBreaker) IsOpen() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state == CircuitOpen
}

// Global circuit breakers for common operations
var (
	dockerCircuitBreaker  *CircuitBreaker
	gitCircuitBreaker     *CircuitBreaker
	networkCircuitBreaker *CircuitBreaker
	circuitBreakerOnce    sync.Once
)

// GetDockerCircuitBreaker returns the global Docker circuit breaker
func GetDockerCircuitBreaker() *CircuitBreaker {
	circuitBreakerOnce.Do(func() {
		dockerCircuitBreaker = NewCircuitBreaker(DefaultCircuitBreakerConfig("docker"))
		gitCircuitBreaker = NewCircuitBreaker(DefaultCircuitBreakerConfig("git"))
		networkCircuitBreaker = NewCircuitBreaker(DefaultCircuitBreakerConfig("network"))
	})
	return dockerCircuitBreaker
}

// GetGitCircuitBreaker returns the global Git circuit breaker
func GetGitCircuitBreaker() *CircuitBreaker {
	circuitBreakerOnce.Do(func() {
		dockerCircuitBreaker = NewCircuitBreaker(DefaultCircuitBreakerConfig("docker"))
		gitCircuitBreaker = NewCircuitBreaker(DefaultCircuitBreakerConfig("git"))
		networkCircuitBreaker = NewCircuitBreaker(DefaultCircuitBreakerConfig("network"))
	})
	return gitCircuitBreaker
}

// GetNetworkCircuitBreaker returns the global network circuit breaker
func GetNetworkCircuitBreaker() *CircuitBreaker {
	circuitBreakerOnce.Do(func() {
		dockerCircuitBreaker = NewCircuitBreaker(DefaultCircuitBreakerConfig("docker"))
		gitCircuitBreaker = NewCircuitBreaker(DefaultCircuitBreakerConfig("git"))
		networkCircuitBreaker = NewCircuitBreaker(DefaultCircuitBreakerConfig("network"))
	})
	return networkCircuitBreaker
}
