package docker

import (
	"context"
	"fmt"
	"time"
)

// IsHostGatewayTarget reports whether `target` points at the host
// rather than a Docker container. Dotted names are treated as host-
// shaped: containers under the default bridge are bare DNS labels,
// and skipping the wait is safer than blocking on a name we can't
// resolve. ADR-025.
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

// WaitForContainer polls (1s tick) until `name` exists in any Docker
// state or `timeout` elapses. ctx cancellation honored. ADR-025.
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

	// First probe is immediate — skip the initial 1s wait.
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
