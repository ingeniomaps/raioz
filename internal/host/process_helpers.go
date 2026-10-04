package host

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// parseCommand parses a command string into command and arguments,
// honoring quotes (see SplitCommand).
func parseCommand(cmdStr string) []string {
	return SplitCommand(cmdStr)
}

// formatEarlyExitError builds the error returned when a host process exits
// inside the settle window. Includes the tail of the stderr log
// so the user sees the real reason without having to dig through log files.
func formatEarlyExitError(name string, window time.Duration, exitErr error, stderrPath string) error {
	tail := ReadLogTail(stderrPath, 8)
	if tail == "" {
		return fmt.Errorf("service %q exited within %s: %w", name, window, exitErr)
	}
	return fmt.Errorf(
		"service %q exited within %s: %w\n--- stderr tail ---\n%s",
		name, window, exitErr, tail,
	)
}

// FormatEarlyExitError is the exported form of formatEarlyExitError, used by
// other packages (orchestrate.HostRunner) that share the settle-window
// behavior.
func FormatEarlyExitError(name string, window time.Duration, exitErr error, stderrPath string) error {
	return formatEarlyExitError(name, window, exitErr, stderrPath)
}

// SettleWindow returns the configured settle window. Exported so other
// packages (orchestrate) use the same default and tests can read it.
func SettleWindow() time.Duration {
	return startSettleWindow
}

// SetSettleWindowForTest overrides the settle window for the duration of a
// test. Returns a restore func the caller must defer. Exported so tests in
// other packages can shrink the window without touching package internals.
func SetSettleWindowForTest(d time.Duration) (restore func()) {
	prev := startSettleWindow
	startSettleWindow = d
	return func() { startSettleWindow = prev }
}

// ReadLogTail returns the last `lines` lines of a file as a single string,
// or empty if the file is missing or unreadable. Best-effort — never errors.
func ReadLogTail(path string, lines int) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	s := strings.TrimRight(string(data), "\n")
	if s == "" {
		return ""
	}
	parts := strings.Split(s, "\n")
	if len(parts) <= lines {
		return s
	}
	return strings.Join(parts[len(parts)-lines:], "\n")
}
