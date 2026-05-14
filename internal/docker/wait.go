package docker

import (
	"context"
	"fmt"
	"time"
)

// IsHostGatewayTarget reports whether the given proxy target points
// at the host (not a Docker container). The launcher-pattern wait
// path (issue 047) skips polling when the target is host-shaped —
// there's no container to wait for. Recognized forms:
//
//   - host.docker.internal (Docker Desktop / rootful Linux gateway)
//   - localhost / 127.0.0.1 / ::1 (host loopback)
//   - any dotted name (looks like an FQDN or IP; container names
//     are bare DNS labels under Docker's default bridge resolver)
//
// Conservative by design: a container literally named "foo.bar"
// is allowed but unusual; treating it as host-shaped is the safer
// default (we skip the wait rather than wait forever on a name we
// can't resolve).
func IsHostGatewayTarget(target string) bool {
	if target == "" {
		return true
	}
	switch target {
	case "host.docker.internal", "localhost", "127.0.0.1", "::1":
		return true
	}
	// FQDNs / IPs contain dots; container names typically don't.
	for _, c := range target {
		if c == '.' {
			return true
		}
	}
	return false
}

// WaitForContainer blocks until a container named `name` exists in
// any Docker state (running, created, restarting, etc.) or until
// timeout. Returns nil when the container appears, an error
// describing the timeout otherwise. ctx cancellation is honored —
// callers that need bounded waits should pass a derived context.
//
// Implemented as a 1s poll loop on top of GetContainerStatusByName;
// we don't have access to the Docker events API stream from this
// layer and the typical wait is single-digit seconds (post-build
// container start), so polling is the right shape.
//
// Used by HostRunner in the launcher-pattern path (issue 047 /
// ADR-025) so `raioz up` doesn't print "ready" while the user's
// `make dev-docker` is still building.
func WaitForContainer(ctx context.Context, name string, timeout time.Duration) error {
	if name == "" {
		return fmt.Errorf("WaitForContainer: empty container name")
	}
	if timeout <= 0 {
		return fmt.Errorf("WaitForContainer: non-positive timeout %v", timeout)
	}

	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	// First check is immediate — the container may already exist.
	if status, _ := GetContainerStatusByName(ctx, name); status != "" {
		return nil
	}

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("WaitForContainer(%s): %w", name, ctx.Err())
		case <-ticker.C:
			if status, _ := GetContainerStatusByName(ctx, name); status != "" {
				return nil
			}
			if time.Now().After(deadline) {
				return fmt.Errorf(
					"WaitForContainer(%s): container did not appear within %v",
					name, timeout,
				)
			}
		}
	}
}
