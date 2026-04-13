package resilience

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestDefaultTimeoutConfig(t *testing.T) {
	cfg := DefaultTimeoutConfig()
	if cfg.DefaultTimeout <= 0 {
		t.Error("DefaultTimeout should be positive")
	}
	if len(cfg.OperationTimeouts) == 0 {
		t.Error("expected operation timeouts")
	}
}

func TestTimeoutConfig_GetTimeout_Known(t *testing.T) {
	cfg := DefaultTimeoutConfig()
	got := cfg.GetTimeout("git.clone")
	if got != 10*time.Minute {
		t.Errorf("expected 10m, got %v", got)
	}
}

func TestTimeoutConfig_GetTimeout_Unknown(t *testing.T) {
	cfg := DefaultTimeoutConfig()
	got := cfg.GetTimeout("unknown.operation")
	if got != cfg.DefaultTimeout {
		t.Errorf("expected default, got %v", got)
	}
}

func TestWithTimeout_Success(t *testing.T) {
	ctx, cancel, err := WithTimeout(context.Background(), 100*time.Millisecond, "test")
	if err != nil {
		t.Fatalf("WithTimeout: %v", err)
	}
	defer cancel()

	if ctx == nil {
		t.Error("expected non-nil ctx")
	}
	_, ok := ctx.Deadline()
	if !ok {
		t.Error("expected deadline set")
	}
}

func TestWithTimeout_NilParent(t *testing.T) {
	ctx, cancel, err := WithTimeout(nil, 100*time.Millisecond, "test")
	if err != nil {
		t.Fatalf("WithTimeout: %v", err)
	}
	defer cancel()
	if ctx == nil {
		t.Error("expected non-nil ctx")
	}
}

func TestWithTimeout_AlreadyCancelled(t *testing.T) {
	parent, cancel := context.WithCancel(context.Background())
	cancel()

	_, _, err := WithTimeout(parent, 100*time.Millisecond, "test")
	if err == nil {
		t.Error("expected error for cancelled parent")
	}
}

func TestExecuteWithTimeout_Success(t *testing.T) {
	err := ExecuteWithTimeout(
		context.Background(), 1*time.Second, "test",
		func(ctx context.Context) error { return nil },
	)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestExecuteWithTimeout_Failure(t *testing.T) {
	myErr := errors.New("my error")
	err := ExecuteWithTimeout(
		context.Background(), 1*time.Second, "test",
		func(ctx context.Context) error { return myErr },
	)
	if err == nil {
		t.Error("expected error")
	}
}

func TestExecuteWithTimeout_Timeout(t *testing.T) {
	err := ExecuteWithTimeout(
		context.Background(), 10*time.Millisecond, "test",
		func(ctx context.Context) error {
			time.Sleep(100 * time.Millisecond)
			return nil
		},
	)
	if err == nil {
		t.Error("expected timeout error")
	}
}

func TestExecuteWithTimeout_CancelledParent(t *testing.T) {
	parent, cancel := context.WithCancel(context.Background())
	cancel()

	err := ExecuteWithTimeout(
		parent, 1*time.Second, "test",
		func(ctx context.Context) error { return nil },
	)
	if err == nil {
		t.Error("expected error for cancelled parent")
	}
}

func TestHandleTimeoutError_Nil(t *testing.T) {
	if err := HandleTimeoutError(context.Background(), nil, "op", time.Second); err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestHandleTimeoutError_Deadline(t *testing.T) {
	err := HandleTimeoutError(
		context.Background(), context.DeadlineExceeded, "op", time.Second,
	)
	if err == nil {
		t.Error("expected error")
	}
}

func TestHandleTimeoutError_CancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := HandleTimeoutError(ctx, context.Canceled, "op", time.Second)
	if err == nil {
		t.Error("expected error")
	}
}

func TestHandleTimeoutError_DeadlineContext(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()
	time.Sleep(5 * time.Millisecond)

	err := HandleTimeoutError(ctx, errors.New("other"), "op", time.Second)
	if err == nil {
		t.Error("expected error")
	}
}

func TestHandleTimeoutError_OtherError(t *testing.T) {
	err := HandleTimeoutError(
		context.Background(), errors.New("other"), "op", time.Second,
	)
	if err == nil {
		t.Error("expected wrapped error")
	}
}

func TestIsTimeoutError_True(t *testing.T) {
	if !IsTimeoutError(context.Background(), context.DeadlineExceeded) {
		t.Error("expected true")
	}
}

func TestIsTimeoutError_Nil(t *testing.T) {
	if IsTimeoutError(context.Background(), nil) {
		t.Error("expected false for nil error")
	}
}

func TestIsTimeoutError_ContextDeadline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()
	time.Sleep(5 * time.Millisecond)

	if !IsTimeoutError(ctx, errors.New("other")) {
		t.Error("expected true when context deadline exceeded")
	}
}

func TestIsCancelledError_True(t *testing.T) {
	if !IsCancelledError(context.Background(), context.Canceled) {
		t.Error("expected true for canceled")
	}
}

func TestIsCancelledError_Nil(t *testing.T) {
	if IsCancelledError(context.Background(), nil) {
		t.Error("expected false for nil")
	}
}

func TestIsCancelledError_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if !IsCancelledError(ctx, errors.New("other")) {
		t.Error("expected true when context cancelled")
	}
}
