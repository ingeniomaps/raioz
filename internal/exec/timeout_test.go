package exec

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestWithTimeout(t *testing.T) {
	ctx, cancel := WithTimeout(100 * time.Millisecond)
	defer cancel()

	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("expected deadline to be set")
	}
	if time.Until(deadline) > 100*time.Millisecond {
		t.Errorf("deadline too far in the future: %v", time.Until(deadline))
	}
}

func TestWithTimeoutFromContext(t *testing.T) {
	parent, parentCancel := context.WithCancel(context.Background())
	defer parentCancel()

	ctx, cancel := WithTimeoutFromContext(parent, 200*time.Millisecond)
	defer cancel()

	if ctx.Err() != nil {
		t.Error("expected context not to be cancelled yet")
	}

	// Cancelling parent should cancel child
	parentCancel()
	time.Sleep(10 * time.Millisecond)
	if ctx.Err() == nil {
		t.Error("expected context to be cancelled after parent cancel")
	}
}

func TestHandleTimeoutError_Nil(t *testing.T) {
	ctx := context.Background()
	if err := HandleTimeoutError(ctx, nil, "op", time.Second); err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestHandleTimeoutError_Timeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()
	time.Sleep(5 * time.Millisecond)

	err := HandleTimeoutError(ctx, context.DeadlineExceeded, "git pull", time.Second)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "git pull") {
		t.Errorf("expected operation name in error: %v", err)
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Errorf("expected 'timed out' in error: %v", err)
	}
}

func TestHandleTimeoutError_OtherError(t *testing.T) {
	ctx := context.Background()
	origErr := errors.New("original error")
	err := HandleTimeoutError(ctx, origErr, "op", time.Second)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, origErr) {
		t.Errorf("expected wrapped original error, got %v", err)
	}
}

func TestIsTimeoutError_True(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()
	time.Sleep(5 * time.Millisecond)

	if !IsTimeoutError(ctx, context.DeadlineExceeded) {
		t.Error("expected true for timeout error")
	}
}

func TestIsTimeoutError_NilError(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()
	time.Sleep(5 * time.Millisecond)

	if IsTimeoutError(ctx, nil) {
		t.Error("expected false for nil error")
	}
}

func TestIsTimeoutError_NonTimeoutError(t *testing.T) {
	ctx := context.Background()
	if IsTimeoutError(ctx, errors.New("other")) {
		t.Error("expected false for non-timeout error")
	}
}

func TestTimeoutConstants(t *testing.T) {
	// Sanity checks on constant values
	if GitCloneTimeout <= 0 {
		t.Error("GitCloneTimeout should be positive")
	}
	if DockerComposeUpTimeout <= 0 {
		t.Error("DockerComposeUpTimeout should be positive")
	}
	if DefaultTimeout <= 0 {
		t.Error("DefaultTimeout should be positive")
	}
}
