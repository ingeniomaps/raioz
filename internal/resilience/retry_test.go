package resilience

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestDefaultRetryConfig(t *testing.T) {
	cfg := DefaultRetryConfig()
	if cfg.MaxAttempts < 1 {
		t.Error("MaxAttempts should be >= 1")
	}
	if cfg.InitialDelay <= 0 {
		t.Error("InitialDelay should be > 0")
	}
	if cfg.BackoffMultiplier <= 1.0 {
		t.Error("BackoffMultiplier should be > 1.0")
	}
}

func TestNetworkRetryConfig(t *testing.T) {
	cfg := NetworkRetryConfig()
	if cfg.MaxAttempts < 1 {
		t.Error("invalid MaxAttempts")
	}
}

func TestGitRetryConfig(t *testing.T) {
	cfg := GitRetryConfig()
	if cfg.MaxAttempts < 1 {
		t.Error("invalid MaxAttempts")
	}
}

func TestDockerRetryConfig(t *testing.T) {
	cfg := DockerRetryConfig()
	if cfg.MaxAttempts < 1 {
		t.Error("invalid MaxAttempts")
	}
}

func TestIsRetryableError_Nil(t *testing.T) {
	if IsRetryableError(nil, nil) {
		t.Error("nil error should not be retryable")
	}
}

func TestIsRetryableError_DeadlineExceeded(t *testing.T) {
	if !IsRetryableError(context.DeadlineExceeded, nil) {
		t.Error("DeadlineExceeded should be retryable")
	}
}

func TestIsRetryableError_Canceled(t *testing.T) {
	if IsRetryableError(context.Canceled, nil) {
		t.Error("Canceled should NOT be retryable")
	}
}

func TestIsRetryableError_TransientPatterns(t *testing.T) {
	patterns := []string{
		"connection refused",
		"connection reset",
		"timeout",
		"temporary failure",
		"network is unreachable",
		"i/o timeout",
		"EOF",
		"broken pipe",
		"rate limit",
		"service unavailable",
		"bad gateway",
	}
	for _, pat := range patterns {
		t.Run(pat, func(t *testing.T) {
			if !IsRetryableError(errors.New(pat), nil) {
				t.Errorf("pattern %q should be retryable", pat)
			}
		})
	}
}

func TestIsRetryableError_NonRetryable(t *testing.T) {
	if IsRetryableError(errors.New("permission denied"), nil) {
		t.Error("permission denied should NOT be retryable")
	}
}

func TestIsRetryableError_CustomRetryable(t *testing.T) {
	myErr := errors.New("my custom error")
	if !IsRetryableError(myErr, []error{myErr}) {
		t.Error("custom error should be retryable")
	}
}

func TestRetry_SuccessFirstAttempt(t *testing.T) {
	cfg := RetryConfig{
		MaxAttempts:       3,
		InitialDelay:      10 * time.Millisecond,
		MaxDelay:          100 * time.Millisecond,
		BackoffMultiplier: 2.0,
	}

	calls := 0
	err := Retry(context.Background(), cfg, "test", func() error {
		calls++
		return nil
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if calls != 1 {
		t.Errorf("expected 1 call, got %d", calls)
	}
}

func TestRetry_SuccessAfterFailure(t *testing.T) {
	cfg := RetryConfig{
		MaxAttempts:       3,
		InitialDelay:      1 * time.Millisecond,
		MaxDelay:          10 * time.Millisecond,
		BackoffMultiplier: 2.0,
	}

	calls := 0
	err := Retry(context.Background(), cfg, "test", func() error {
		calls++
		if calls < 2 {
			return errors.New("timeout")
		}
		return nil
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if calls != 2 {
		t.Errorf("expected 2 calls, got %d", calls)
	}
}

func TestRetry_ExhaustAttempts(t *testing.T) {
	cfg := RetryConfig{
		MaxAttempts:       3,
		InitialDelay:      1 * time.Millisecond,
		MaxDelay:          10 * time.Millisecond,
		BackoffMultiplier: 2.0,
	}

	calls := 0
	err := Retry(context.Background(), cfg, "test", func() error {
		calls++
		return errors.New("timeout")
	})
	if err == nil {
		t.Error("expected error after exhausting attempts")
	}
	if calls != 3 {
		t.Errorf("expected 3 calls, got %d", calls)
	}
}

func TestRetry_NonRetryableError(t *testing.T) {
	cfg := RetryConfig{
		MaxAttempts:       3,
		InitialDelay:      1 * time.Millisecond,
		MaxDelay:          10 * time.Millisecond,
		BackoffMultiplier: 2.0,
	}

	calls := 0
	err := Retry(context.Background(), cfg, "test", func() error {
		calls++
		return errors.New("permission denied")
	})
	if err == nil {
		t.Error("expected error")
	}
	if calls != 1 {
		t.Errorf("expected 1 call (non-retryable), got %d", calls)
	}
}

func TestRetry_ContextCancelled(t *testing.T) {
	cfg := RetryConfig{
		MaxAttempts:       5,
		InitialDelay:      100 * time.Millisecond,
		MaxDelay:          1 * time.Second,
		BackoffMultiplier: 2.0,
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := Retry(ctx, cfg, "test", func() error {
		return errors.New("timeout")
	})
	if err == nil {
		t.Error("expected cancellation error")
	}
}

func TestRetry_NilContext(t *testing.T) {
	cfg := RetryConfig{
		MaxAttempts:       2,
		InitialDelay:      1 * time.Millisecond,
		MaxDelay:          10 * time.Millisecond,
		BackoffMultiplier: 2.0,
	}
	// nil context should work
	err := Retry(nil, cfg, "test", func() error { return nil })
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRetryWithContext_Success(t *testing.T) {
	cfg := RetryConfig{
		MaxAttempts:       3,
		InitialDelay:      1 * time.Millisecond,
		MaxDelay:          10 * time.Millisecond,
		BackoffMultiplier: 2.0,
	}

	calls := 0
	err := RetryWithContext(
		context.Background(), cfg, "test",
		func(ctx context.Context) error {
			calls++
			return nil
		},
	)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if calls != 1 {
		t.Errorf("expected 1 call, got %d", calls)
	}
}

func TestRetryWithContext_ExhaustAttempts(t *testing.T) {
	cfg := RetryConfig{
		MaxAttempts:       2,
		InitialDelay:      1 * time.Millisecond,
		MaxDelay:          10 * time.Millisecond,
		BackoffMultiplier: 2.0,
	}

	err := RetryWithContext(
		context.Background(), cfg, "test",
		func(ctx context.Context) error {
			return errors.New("timeout")
		},
	)
	if err == nil {
		t.Error("expected error")
	}
}

func TestRetryWithContext_NonRetryable(t *testing.T) {
	cfg := RetryConfig{
		MaxAttempts:       3,
		InitialDelay:      1 * time.Millisecond,
		MaxDelay:          10 * time.Millisecond,
		BackoffMultiplier: 2.0,
	}

	calls := 0
	err := RetryWithContext(
		context.Background(), cfg, "test",
		func(ctx context.Context) error {
			calls++
			return errors.New("fatal")
		},
	)
	if err == nil {
		t.Error("expected error")
	}
	if calls != 1 {
		t.Errorf("expected 1 call, got %d", calls)
	}
}

func TestRetryWithContext_CancelDuringWait(t *testing.T) {
	cfg := RetryConfig{
		MaxAttempts:       5,
		InitialDelay:      500 * time.Millisecond,
		MaxDelay:          1 * time.Second,
		BackoffMultiplier: 2.0,
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	err := RetryWithContext(ctx, cfg, "test", func(c context.Context) error {
		return errors.New("timeout")
	})
	if err == nil {
		t.Error("expected cancellation error")
	}
}
