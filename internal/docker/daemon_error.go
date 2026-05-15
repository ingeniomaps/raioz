package docker

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"raioz/internal/domain/interfaces"
)

// daemonDownSignatures matches the docker CLI / engine API prose that
// signals "daemon is not reachable." Substring-based heuristic; the
// match strings are stable across docker versions. Stays inside the
// docker package so the app layer reads a typed error
// (interfaces.ErrDaemonUnreachable) instead of stdout strings.
var daemonDownSignatures = []string{
	"cannot connect to the docker daemon",
	"connection refused",
	"no such host",
	"is the docker daemon running",
	"dial unix /var/run/docker.sock",
}

// wrapDaemonError checks raw exec output + the original error and, if
// the docker CLI signaled daemon-down, wraps the error so callers can
// errors.Is(err, interfaces.ErrDaemonUnreachable). All other failures
// pass through with `%w` so existing error inspection keeps working.
func wrapDaemonError(op string, output []byte, err error) error {
	if err == nil {
		return nil
	}
	haystack := strings.ToLower(string(output) + " " + err.Error())
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		haystack += " " + strings.ToLower(string(exitErr.Stderr))
	}
	for _, sig := range daemonDownSignatures {
		if strings.Contains(haystack, sig) {
			// Two-wrap: outer carries the op + original error; inner
			// is the sentinel so callers branch via errors.Is.
			return fmt.Errorf("%s: %w (%w)", op, interfaces.ErrDaemonUnreachable, err)
		}
	}
	return fmt.Errorf("%s: %w", op, err)
}
