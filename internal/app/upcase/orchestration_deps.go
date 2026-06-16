package upcase

import (
	"fmt"

	"raioz/internal/domain/interfaces"
	"raioz/internal/domain/models"
	"raioz/internal/netutil"
)

// resolveDepPublishPorts decides which `ports:` list to pass to ImageRunner
// for a given dependency. Extracted from processOrchestration both for
// readability and to keep orchestration.go under the 400-line cap.
//
// Precedence:
//
//  1. Allocator result for the dep. Covers both the new `publish:` /
//     `expose:` path AND legacy `ports:` that the allocator could parse
//     (issue 020) — it represents the fresh, conflict-checked assignment
//     with a deterministic host port.
//  2. Legacy `ports:` list the allocator could not model (ranges,
//     IP-prefixed, /proto). Passed through verbatim for backwards
//     compatibility — a deprecation warning is emitted elsewhere.
//  3. Empty — the dep runs internal-only. ImageRunner will not emit a
//     `ports:` key in the generated compose file, so the container is only
//     reachable from inside the Docker network.
func resolveDepPublishPorts(
	name string,
	entry models.InfraEntry,
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

// applyInlineDepEndpoint fills the URL scheme and (for legacy `ports:` the
// allocator could not handle) the container/host ports of an inline infra
// endpoint.
//
//   - Scheme comes from the image so <DEP>_URL is redis://, postgresql://,
//     etc. instead of a useless http:// (issue 020, defect B).
//   - When the allocator already mapped this dep (`allocated`), its
//     conflict-checked host port from buildEndpoints stands. Only the
//     unparseable legacy fallback copies the container port as the host
//     port — historic best-effort behavior, still wrong for host callers
//     but no worse than before.
func applyInlineDepEndpoint(
	ep *interfaces.ServiceEndpoint,
	name string,
	deps *models.Deps,
	portAllocs *PortAllocResult,
) {
	entry, ok := deps.Infra[name]
	if !ok || entry.Inline == nil {
		return
	}
	if entry.Inline.Image != "" {
		ep.Scheme = netutil.SchemeForImage(entry.Inline.Image)
	}
	allocated := false
	if portAllocs != nil {
		_, allocated = portAllocs.Deps[name]
	}
	if len(entry.Inline.Ports) > 0 && entry.Inline.Publish == nil && !allocated {
		ep.Port = parseFirstPort(entry.Inline.Ports[0])
		ep.HostPort = ep.Port
	}
}
