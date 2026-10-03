package docker

import (
	"context"
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"strings"

	exectimeout "raioz/internal/exec"
	"raioz/internal/naming"
	"raioz/internal/runtime"
)

// PublishedPort is a host port a raioz-managed container publishes.
type PublishedPort struct {
	Container string
	Project   string // empty for a workspace-shared dependency (ADR-002)
	Workspace string
	Service   string
	HostPort  int
}

// ListManagedPublishedPorts returns every host port published by a running
// raioz-managed container, whatever project or workspace it belongs to.
func ListManagedPublishedPorts(ctx context.Context) ([]PublishedPort, error) {
	timeoutCtx, cancel := exectimeout.WithTimeoutFromContext(ctx, exectimeout.DockerInspectTimeout)
	defer cancel()

	format := "{{.Names}}|" +
		"{{.Label \"" + naming.LabelProject + "\"}}|" +
		"{{.Label \"" + naming.LabelWorkspace + "\"}}|" +
		"{{.Label \"" + naming.LabelService + "\"}}|" +
		"{{.Ports}}"
	out, err := exec.CommandContext(timeoutCtx, runtime.Binary(), "ps",
		"--filter", "label="+naming.LabelManaged+"=true",
		"--format", format).Output()
	if err != nil {
		return nil, fmt.Errorf("docker ps failed: %w", err)
	}

	var ports []PublishedPort
	for _, line := range strings.Split(string(out), "\n") {
		f := strings.SplitN(strings.TrimSpace(line), "|", 5)
		if len(f) < 5 {
			continue
		}
		for _, hostPort := range parsePublishedHostPorts(f[4]) {
			ports = append(ports, PublishedPort{
				Container: f[0], Project: f[1], Workspace: f[2], Service: f[3], HostPort: hostPort,
			})
		}
	}
	return ports, nil
}

// parsePublishedHostPorts extracts the host side of every published
// mapping in a `docker ps` Ports column, deduplicated and sorted. An entry
// with no "->" is a port the container exposes but does not publish.
//
//	0.0.0.0:36379->6379/tcp, [::]:36379->6379/tcp, 8025/tcp  →  [36379]
func parsePublishedHostPorts(column string) []int {
	seen := map[int]bool{}
	for _, entry := range strings.Split(column, ",") {
		hostSide, _, published := strings.Cut(strings.TrimSpace(entry), "->")
		if !published {
			continue
		}
		i := strings.LastIndexByte(hostSide, ':')
		if i < 0 {
			continue
		}
		// A range (8000-8002) is listed by its first port only: raioz
		// never publishes ranges, and a foreign one is still a holder.
		first, _, _ := strings.Cut(hostSide[i+1:], "-")
		if port, err := strconv.Atoi(first); err == nil && port > 0 {
			seen[port] = true
		}
	}
	ports := make([]int, 0, len(seen))
	for port := range seen {
		ports = append(ports, port)
	}
	sort.Ints(ports)
	return ports
}
