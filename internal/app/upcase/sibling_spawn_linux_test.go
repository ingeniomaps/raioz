//go:build linux

package upcase

import (
	"syscall"
	"testing"
)

// TestSetPdeathsig guards ADR-026 on Linux: spawned
// siblings must be wired to receive SIGTERM when the parent
// process exits, so Ctrl+C on the parent doesn't leak the child
// raioz tree.
func TestSetPdeathsig(t *testing.T) {
	attr := &syscall.SysProcAttr{}
	setPdeathsig(attr)

	if attr.Pdeathsig != syscall.SIGTERM {
		t.Errorf("Pdeathsig = %v, want SIGTERM (%v)",
			attr.Pdeathsig, syscall.SIGTERM)
	}
}

// TestSetPdeathsig_NilSafe is the smoke check: passing a fresh
// SysProcAttr must not panic. (The platform fallback file passes a
// nil-receiver no-op; the Linux file mutates a real field.)
func TestSetPdeathsig_NilSafe(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("setPdeathsig panicked on fresh attr: %v", r)
		}
	}()
	setPdeathsig(&syscall.SysProcAttr{})
}
