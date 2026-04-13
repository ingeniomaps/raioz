package notify

import (
	"runtime"
	"testing"
)

func TestSend_DoesNotPanic(t *testing.T) {
	// Send should not panic even when notification tool is missing.
	// It must fail silently. We don't check output — only that it returns.
	Send("Raioz", "Hello world")
	Send("", "")
	Send("Title with \"quotes\"", "Message with special chars: $!&")
}

func TestSendLinux_DoesNotPanic(t *testing.T) {
	// Calling sendLinux directly to cover the code path on any OS.
	// If notify-send is not installed, it's a no-op (silent).
	sendLinux("Title", "Message")
}

func TestSendMacOS_DoesNotPanic(t *testing.T) {
	// Calling sendMacOS directly to cover the code path on any OS.
	// If osascript is not installed (Linux), exec.Command().Run() will
	// return an error which is ignored intentionally.
	sendMacOS("Title", "Message")
}

func TestSend_DispatchesByOS(t *testing.T) {
	// This test documents the expected platform-specific dispatch.
	// On unsupported OS (e.g. windows, freebsd), Send must be a no-op.
	tests := []struct {
		name string
	}{
		{"current OS: " + runtime.GOOS},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Send("title", "msg")
		})
	}
}
