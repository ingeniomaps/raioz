package upcase

import (
	"fmt"

	"raioz/internal/config"
)

// resolveDepPublishPorts decides which `ports:` list to pass to ImageRunner
// for a given dependency. Extracted from processOrchestration both for
// readability and to keep orchestration.go under the 400-line cap.
//
// Precedence:
//
//  1. Allocator result for the dep (new `publish:` / `expose:` path). Takes
//     priority because it represents the fresh, conflict-checked assignment.
//  2. Legacy `ports:` list on the inline infra. Passed through verbatim for
//     backwards compatibility — a deprecation warning is emitted elsewhere.
//  3. Empty — the dep runs internal-only. ImageRunner will not emit a
//     `ports:` key in the generated compose file, so the container is only
//     reachable from inside the Docker network.
func resolveDepPublishPorts(
	name string,
	entry config.InfraEntry,
	portAllocs *PortAllocResult,
) []string {
	if portAllocs != nil {
		if alloc, ok := portAllocs.Deps[name]; ok && len(alloc.Mappings) > 0 {
			out := make([]string, 0, len(alloc.Mappings))
			for _, m := range alloc.Mappings {
				out = append(out, fmt.Sprintf("%d:%d", m.HostPort, m.ContainerPort))
			}
			return out
		}
	}
	if entry.Inline != nil && len(entry.Inline.Ports) > 0 {
		return entry.Inline.Ports
	}
	return nil
}
