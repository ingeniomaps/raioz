package exec

import (
	"context"
	"fmt"
	"time"
)

// Timeout durations for different operations
const (
	// Git operations
	GitCloneTimeout    = 10 * time.Minute
	GitPullTimeout     = 5 * time.Minute
	GitCheckoutTimeout = 2 * time.Minute

	// Docker operations
	DockerComposeUpTimeout   = 5 * time.Minute
	DockerComposeDownTimeout = 2 * time.Minute
	DockerPullTimeout        = 15 * time.Minute
	DockerInspectTimeout     = 30 * time.Second
	DockerStatusTimeout      = 30 * time.Second
	DockerNetworkTimeout     = 30 * time.Second
	DockerVolumeTimeout      = 30 * time.Second
	DockerLogsTimeout        = 2 * time.Minute
	DockerStatsTimeout       = 30 * time.Second

	// General operations
	DefaultTimeout = 5 * time.Minute
)

// WithTimeout creates a context with the specified timeout
func WithTimeout(timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), timeout)
}

// WithTimeoutFromContext creates a context with timeout from an existing context
func WithTimeoutFromContext(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, timeout)
}

// HandleTimeoutError checks if an error is due to timeout and formats it appropriately
func HandleTimeoutError(ctx context.Context, err error, operation string, timeout time.Duration) error {
	if err == nil {
		return nil
	}

	if ctx.Err() == context.DeadlineExceeded {
		return fmt.Errorf("%s timed out after %v", operation, timeout)
	}

	return fmt.Errorf("%s failed: %w", operation, err)
}

// IsTimeoutError checks if an error is due to timeout
func IsTimeoutError(ctx context.Context, err error) bool {
	return err != nil && ctx.Err() == context.DeadlineExceeded
}
