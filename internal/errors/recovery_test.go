package errors

import (
	"testing"
)

// Note: RecoverPanic and RecoverPanicWithError use recover() indirectly,
// which per the Go spec only works when called DIRECTLY from a deferred
// function. So we can only test the "no panic" path for those helpers.
// SafeExecute uses direct recover(), so its panic-recovery paths work.

func TestRecoverPanic_NoPanicPath(t *testing.T) {
	// Without an in-flight panic, RecoverPanic returns nil
	result := RecoverPanic("test-op")
	if result != nil {
		t.Error("expected nil without panic")
	}
}
