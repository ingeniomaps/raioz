package docker

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	exectimeout "raioz/internal/exec"
	"raioz/internal/runtime"
)

// GetPublishedHostPort returns the host port a running container has published
// for the given container port (e.g. container 6379 → host 6379, or a remapped
// host port). It shells out to `docker port <name> <containerPort>/tcp` and
// parses the host side of the first binding (`0.0.0.0:6379` → 6379).
//
// Returns 0 with no error when the container is not running, the port is not
// published, or docker can't be reached. Callers treat 0 as "no live port —
// fall back to normal allocation". An error is returned only on timeout.
//
// Used to keep a workspace-shared dependency on a single host port across
// projects: the first `up` publishes it, later consumers read the live value
// here instead of auto-assigning a divergent one (ADR-002, issue 020).
func GetPublishedHostPort(ctx context.Context, name string, containerPort int) (int, error) {
	if name == "" || containerPort <= 0 {
		return 0, nil
	}

	timeoutCtx, cancel := exectimeout.WithTimeoutFromContext(ctx, exectimeout.DockerInspectTimeout)
	defer cancel()

	spec := fmt.Sprintf("%d/tcp", containerPort)
	cmd := exec.CommandContext(timeoutCtx, runtime.Binary(), "port", name, spec)
	out, err := cmd.Output()
	if err != nil {
		if exectimeout.IsTimeoutError(timeoutCtx, err) {
			return 0, fmt.Errorf("docker port timed out after %v", exectimeout.DockerInspectTimeout)
		}
		// Container not running / port not published / docker unreachable.
		return 0, nil
	}

	return parsePublishedHostPort(string(out)), nil
}

// parsePublishedHostPort extracts the host port from `docker port` output.
// Docker prints one line per binding, e.g.:
//
//	0.0.0.0:6379
//	[::]:6379
//
// It returns the first parseable host port. IPv6 lines like `[::]:6379` are
// handled by splitting on the last colon.
func parsePublishedHostPort(out string) int {
	for line := range strings.SplitSeq(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		idx := strings.LastIndex(line, ":")
		if idx < 0 || idx == len(line)-1 {
			continue
		}
		port, err := strconv.Atoi(line[idx+1:])
		if err == nil && port > 0 {
			return port
		}
	}
	return 0
}
