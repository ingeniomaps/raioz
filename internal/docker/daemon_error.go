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
//
// Verified against:
//   - docker 24.x / 25.x CLI
//   - podman 4.x / 5.x (uses "unable to connect to Podman" and
//     "podman.sock" prose)
//   - nerdctl 1.x / 2.x (delegates to containerd; prose includes
//     "containerd is not running" and "dial unix … containerd.sock")
//
// When the runtime abstraction (`internal/runtime/`) adds a new
// runtime, add a fixture to daemon_error_test.go and any missing
// substring here.
var daemonDownSignatures = []string{
	// Docker
	"cannot connect to the docker daemon",
	"connection refused",
	"no such host",
	"is the docker daemon running",
	"dial unix /var/run/docker.sock",
	// Podman (podman ps with daemon down)
	"unable to connect to podman",
	"cannot connect to podman",
	"podman.sock",
	// nerdctl / containerd
	"containerd is not running",
	"containerd.sock",
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
