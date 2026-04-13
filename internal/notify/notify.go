// Package notify sends desktop notifications across platforms.
package notify

import (
	"os/exec"
	"runtime"
	"testing"
)

// Send sends a desktop notification. Fails silently if the
// notification tool is not installed. It is a no-op when running
// under `go test` to avoid spamming the developer's desktop with
// test fixtures.
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
	// Try notify-send (most Linux desktops)
	if path, err := exec.LookPath("notify-send"); err == nil {
		// nolint:errcheck // desktop notifications are best-effort;
		// we must not fail raioz if notify-send is missing or broken.
		exec.Command(path, title, message).Run()
	}
}

func sendMacOS(title, message string) {
	script := `display notification "` + message +
		`" with title "` + title + `"`
	// nolint:errcheck // desktop notifications are best-effort; we must not
	// fail raioz if osascript is missing (e.g. running under Linux CI).
	exec.Command("osascript", "-e", script).Run()
}
