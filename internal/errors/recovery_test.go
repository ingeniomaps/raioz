package errors

import (
	stderrors "errors"
	"strings"
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

func TestRecoverPanicWithError_NoPanicPath(t *testing.T) {
	err := RecoverPanicWithError("test-op")
	if err != nil {
		t.Error("expected nil without panic")
	}
}

func TestSafeExecute_NoError(t *testing.T) {
	err := SafeExecute("test", func() error { return nil })
	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestSafeExecute_ReturnsError(t *testing.T) {
	orig := stderrors.New("returned")
	err := SafeExecute("test", func() error { return orig })
	if err != orig {
		t.Errorf("expected returned error, got %v", err)
	}
}

func TestSafeExecute_PanicString(t *testing.T) {
	err := SafeExecute("test", func() error {
		panic("panic string")
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "panic string") {
		t.Errorf("expected panic message: %v", err)
	}
}

func TestSafeExecute_PanicError(t *testing.T) {
	err := SafeExecute("test", func() error {
		panic(stderrors.New("panic error"))
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "panic error") {
		t.Errorf("expected panic error message: %v", err)
	}
}

func TestSafeExecute_PanicReturnsRaiozError(t *testing.T) {
	err := SafeExecute("my-op", func() error {
		panic("kaboom")
	})
	var target *RaiozError
	if !As(err, &target) {
		t.Fatal("expected RaiozError from panic")
	}
	if target.Code != ErrCodeInternalError {
		t.Errorf("expected internal error code, got %s", target.Code)
	}
	if target.Context["operation"] != "my-op" {
		t.Error("expected operation in context")
	}
}

func TestGetCallerInfo(t *testing.T) {
	info := GetCallerInfo(0)
	if info == nil {
		t.Fatal("expected non-nil info")
	}
	// Should contain either function info or caller=unknown
	if _, ok := info["function"]; !ok {
		if _, ok := info["caller"]; !ok {
			t.Error("expected function or caller key")
		}
	}
}

func TestGetCallerInfo_HighSkip(t *testing.T) {
	info := GetCallerInfo(1000)
	if info == nil {
		t.Fatal("expected non-nil info")
	}
	// May return caller: unknown for out-of-range skip
}
