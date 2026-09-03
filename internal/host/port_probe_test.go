package host

import (
	"context"
	"net"
	"testing"
)

func TestIsPortListening(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port

	if !IsPortListening(t.Context(), port) {
		t.Errorf("IsPortListening(%d) = false with a listener bound, want true", port)
	}

	// Same port, nobody serving: this is the case that separates "the
	// wrapper PID is alive" from "the service is attending".
	if err := ln.Close(); err != nil {
		t.Fatalf("close listener: %v", err)
	}
	if IsPortListening(t.Context(), port) {
		t.Errorf("IsPortListening(%d) = true after the listener closed, want false", port)
	}
}

func TestIsPortListening_NoPortDeclared(t *testing.T) {
	cases := []struct {
		name string
		port int
	}{
		{"zero", 0},
		{"negative", -1},
		{"above range", 70000},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if IsPortListening(t.Context(), tc.port) {
				t.Errorf("IsPortListening(%d) = true, want false", tc.port)
			}
		})
	}
}

// A cancelled context must not leave status blocked on a dial. The port is
// genuinely served here, so only the cancellation can produce false.
func TestIsPortListening_CancelledContext(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = ln.Close() }()
	port := ln.Addr().(*net.TCPAddr).Port

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if IsPortListening(ctx, port) {
		t.Error("IsPortListening = true with a cancelled context, want false")
	}
}
