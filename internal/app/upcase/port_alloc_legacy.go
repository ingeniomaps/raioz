package upcase

import (
	"fmt"
	"strconv"
	"strings"

	"raioz/internal/domain/models"
)

// allocLegacyPortsDeps handles pass 5: dependencies still using the legacy
// `ports:` field (pre-publish/expose). A single-value entry like
// `ports: ["6379"]` tells Docker to publish container port 6379 on a RANDOM
// host port — which raioz could never report back to host callers, so it
// emitted localhost:<container-port> (unreachable). See issue 020 defect A.
//
// Routing these through the allocator gives them a deterministic,
// conflict-checked host port, exactly like `publish:`. resolveDepPublishPorts
// then emits `host:container` and buildEndpoints reads the real host port.
// Specs the allocator cannot model (ranges, IP-prefixed, /proto) are left
// untouched and fall through to the verbatim legacy path.
func allocLegacyPortsDeps(
	deps *models.Deps,
	depNames []string,
	taken map[int]string,
	result *PortAllocResult,
) error {
	for _, name := range depNames {
		if _, done := result.Deps[name]; done {
			continue
		}
		entry := deps.Infra[name]
		if entry.Inline == nil || entry.Inline.Publish != nil {
			continue
		}
		if len(entry.Inline.Ports) == 0 || entry.Inline.Project != "" {
			continue
		}
		mappings, err := assignLegacyDepPorts(name, entry.Inline.Ports, taken)
		if err != nil {
			return err
		}
		if len(mappings) == 0 {
			continue // unmodelable specs — leave to the verbatim legacy path
		}
		result.Deps[name] = DepPortAllocation{
			Name:     name,
			Mappings: mappings,
			Explicit: false,
		}
	}
	return nil
}

// assignLegacyDepPorts resolves each legacy `ports:` entry into a concrete
// host→container mapping. "6379" → container 6379, host allocated near it
// (negotiable, like an implicit port). "8080:6379" → host 8080 pinned
// (hard-fail on conflict, matching Docker's own behavior), container 6379.
//
// All entries are parsed before any port is claimed: if even one entry uses
// a shape raioz doesn't model, the whole dep is left to the verbatim legacy
// path (returns nil, nil) rather than partially rewriting its port list.
func assignLegacyDepPorts(
	name string,
	ports []string,
	taken map[int]string,
) ([]DepPortMapping, error) {
	type parsed struct {
		host, container int
		pinned          bool
	}
	specs := make([]parsed, 0, len(ports))
	for _, p := range ports {
		host, container, pinned, ok := parseLegacyPortSpec(p)
		if !ok {
			return nil, nil
		}
		specs = append(specs, parsed{host, container, pinned})
	}

	owner := fmt.Sprintf("dep '%s'", name)
	mappings := make([]DepPortMapping, 0, len(specs))
	for _, s := range specs {
		final := s.host
		if s.pinned {
			if holder, clash := taken[s.host]; clash {
				return nil, portConflictExplicitError(owner, holder, s.host)
			}
		} else {
			fp, err := findFreePort(s.container, taken, owner)
			if err != nil {
				return nil, err
			}
			final = fp
		}
		taken[final] = owner
		mappings = append(mappings, DepPortMapping{
			HostPort:      final,
			ContainerPort: s.container,
		})
	}
	return mappings, nil
}

// parseLegacyPortSpec splits a legacy `ports:` entry into host and container
// ports. "8080:6379" → (8080, 6379, pinned=true). "6379" → (0, 6379,
// pinned=false): no host port chosen, the allocator picks one. ok is false
// for shapes raioz does not model (port ranges, IP-prefixed bindings,
// /proto suffixes) so the caller can fall back to the verbatim legacy path.
func parseLegacyPortSpec(spec string) (host, container int, pinned, ok bool) {
	parts := strings.Split(strings.TrimSpace(spec), ":")
	switch len(parts) {
	case 1:
		c := atoiPort(parts[0])
		if c <= 0 {
			return 0, 0, false, false
		}
		return 0, c, false, true
	case 2:
		h, c := atoiPort(parts[0]), atoiPort(parts[1])
		if h <= 0 || c <= 0 {
			return 0, 0, false, false
		}
		return h, c, true, true
	default:
		return 0, 0, false, false
	}
}

// atoiPort parses a bare port number, rejecting anything that is not a clean
// positive integer (e.g. "6379/tcp", "6379-6381", "0.0.0.0"). Such values
// signal a spec shape the legacy allocator path intentionally declines.
func atoiPort(s string) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil || n <= 0 {
		return 0
	}
	return n
}
