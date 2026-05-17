// Package notify sends desktop notifications across platforms.
package notify

import (
	"context"
	"os/exec"
	"runtime"
	"testing"
	"time"
)

// sendTimeout bounds each notification exec. Stale D-Bus (SSH with
// dead X forwarding, daemon wedged) and macOS Notification Center
// hangs can otherwise pin a watch-mode goroutine indefinitely. Issue
// 043. 2s is comfortably above the worst-case healthy notify-send
// latency (~150ms) while still snappy enough to drop quietly.
const sendTimeout = 2 * time.Second

// Send sends a desktop notification. Fails silently if the
// notification tool is not installed, is broken, or hangs. It is a
// no-op when running under `go test` to avoid spamming the developer's
// desktop with test fixtures.
func Send(title, message string) {
	if testing.Testing() {
		return
	}
	switch runtime.GOOS {
	case "linux":
		sendLinux(title, message)
	case "darwin":
		sendMacOS(title, message)
	}
}

func sendLinux(title, message string) {
	path, err := exec.LookPath("notify-send")
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), sendTimeout)
	defer cancel()
	// nolint:errcheck // desktop notifications are best-effort; we drop
	// silently on any failure including timeout (issue 043).
	exec.CommandContext(ctx, path, title, message).Run()
}

func sendMacOS(title, message string) {
	script := `display notification "` + message +
		`" with title "` + title + `"`
	ctx, cancel := context.WithTimeout(context.Background(), sendTimeout)
	defer cancel()
	// nolint:errcheck // best-effort; same rationale as sendLinux.
	exec.CommandContext(ctx, "osascript", "-e", script).Run()
}
