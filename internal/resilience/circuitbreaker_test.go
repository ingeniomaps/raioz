package resilience

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCircuitState_String(t *testing.T) {
	cases := []struct {
		s    CircuitState
		want string
	}{
		{CircuitClosed, "closed"},
		{CircuitOpen, "open"},
		{CircuitHalfOpen, "half-open"},
		{CircuitState(99), "unknown"},
	}
	for _, tc := range cases {
		if got := tc.s.String(); got != tc.want {
			t.Errorf("got %q, want %q", got, tc.want)
		}
	}
}

func TestDefaultCircuitBreakerConfig(t *testing.T) {
	cfg := DefaultCircuitBreakerConfig("test")
	if cfg.Name != "test" {
		t.Errorf("name mismatch")
	}
	if cfg.FailureThreshold <= 0 {
		t.Error("FailureThreshold should be positive")
	}
	if cfg.Timeout <= 0 {
		t.Error("Timeout should be positive")
	}
}

func TestNewCircuitBreaker(t *testing.T) {
	cb := NewCircuitBreaker(DefaultCircuitBreakerConfig("test"))
	if cb == nil {
		t.Fatal("NewCircuitBreaker returned nil")
	}
	if cb.GetState() != CircuitClosed {
		t.Error("expected initial state to be closed")
	}
	if cb.IsOpen() {
		t.Error("expected not open initially")
	}
}

func TestCircuitBreaker_Execute_Success(t *testing.T) {
	cb := NewCircuitBreaker(DefaultCircuitBreakerConfig("test"))
	err := cb.Execute(context.Background(), "op", func() error { return nil })
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if cb.GetState() != CircuitClosed {
		t.Error("should still be closed after success")
	}
}

func TestCircuitBreaker_Execute_FailureThreshold(t *testing.T) {
	cfg := CircuitBreakerConfig{
		FailureThreshold: 3,
		SuccessThreshold: 2,
		Timeout:          1 * time.Second,
		Name:             "test",
	}
	cb := NewCircuitBreaker(cfg)

	failingFn := func() error { return errors.New("fail") }

	// Fail 3 times to open the circuit
	for i := 0; i < 3; i++ {
		_ = cb.Execute(context.Background(), "op", failingFn)
	}

	if !cb.IsOpen() {
		t.Error("expected circuit to be open after threshold failures")
	}

	// Next call should be blocked
	err := cb.Execute(context.Background(), "op", failingFn)
	if err == nil {
		t.Error("expected error — circuit is open")
	}
}

func TestCircuitBreaker_HalfOpen_Success(t *testing.T) {
	cfg := CircuitBreakerConfig{
		FailureThreshold: 2,
		SuccessThreshold: 2,
		Timeout:          10 * time.Millisecond,
		Name:             "test",
	}
	cb := NewCircuitBreaker(cfg)

	failingFn := func() error { return errors.New("fail") }
	successFn := func() error { return nil }

	// Trip the breaker
	for i := 0; i < 2; i++ {
		_ = cb.Execute(context.Background(), "op", failingFn)
	}
	if !cb.IsOpen() {
		t.Fatal("expected to be open")
	}

	// Wait for timeout to allow half-open
	time.Sleep(20 * time.Millisecond)

	// Successful calls should transition to closed
	for i := 0; i < 2; i++ {
		if err := cb.Execute(context.Background(), "op", successFn); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}

	if cb.GetState() != CircuitClosed {
		t.Errorf("expected closed, got %s", cb.GetState())
	}
}

func TestCircuitBreaker_HalfOpen_FailReopens(t *testing.T) {
	cfg := CircuitBreakerConfig{
		FailureThreshold: 2,
		SuccessThreshold: 2,
		Timeout:          10 * time.Millisecond,
		Name:             "test",
	}
	cb := NewCircuitBreaker(cfg)

	failingFn := func() error { return errors.New("fail") }

	// Trip the breaker
	for i := 0; i < 2; i++ {
		_ = cb.Execute(context.Background(), "op", failingFn)
	}

	// Wait for half-open
	time.Sleep(20 * time.Millisecond)

	// One failure in half-open should reopen
	_ = cb.Execute(context.Background(), "op", failingFn)
	if !cb.IsOpen() {
		t.Error("expected to reopen after half-open failure")
	}
}

func TestCircuitBreaker_Reset(t *testing.T) {
	cfg := CircuitBreakerConfig{
		FailureThreshold: 2,
		SuccessThreshold: 1,
		Timeout:          1 * time.Second,
		Name:             "test",
	}
	cb := NewCircuitBreaker(cfg)

	// Trip it
	for i := 0; i < 2; i++ {
		_ = cb.Execute(context.Background(), "op", func() error { return errors.New("fail") })
	}
	if !cb.IsOpen() {
		t.Fatal("expected to be open")
	}

	cb.Reset()
	if cb.GetState() != CircuitClosed {
		t.Error("expected closed after reset")
	}
}

func TestCircuitBreaker_ExecuteWithContext_Success(t *testing.T) {
	cb := NewCircuitBreaker(DefaultCircuitBreakerConfig("test"))
	err := cb.ExecuteWithContext(
		context.Background(), "op",
		func(ctx context.Context) error { return nil },
	)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCircuitBreaker_ExecuteWithContext_Failure(t *testing.T) {
	cfg := CircuitBreakerConfig{
		FailureThreshold: 2,
		SuccessThreshold: 1,
		Timeout:          1 * time.Second,
		Name:             "test",
	}
	cb := NewCircuitBreaker(cfg)

	failingFn := func(ctx context.Context) error {
		return errors.New("fail")
	}

	for i := 0; i < 2; i++ {
		_ = cb.ExecuteWithContext(context.Background(), "op", failingFn)
	}

	if !cb.IsOpen() {
		t.Error("expected open")
	}
}

func TestGetDockerCircuitBreaker(t *testing.T) {
	cb := GetDockerCircuitBreaker()
	if cb == nil {
		t.Fatal("expected non-nil")
	}
}

func TestGetGitCircuitBreaker(t *testing.T) {
	cb := GetGitCircuitBreaker()
	if cb == nil {
		t.Fatal("expected non-nil")
	}
}

func TestGetNetworkCircuitBreaker(t *testing.T) {
	cb := GetNetworkCircuitBreaker()
	if cb == nil {
		t.Fatal("expected non-nil")
	}
}
