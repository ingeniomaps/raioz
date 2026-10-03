package upcase

import (
	"sort"

	"raioz/internal/domain/models"
)

// WantedPort is a host port a project asks for.
type WantedPort struct {
	Port int
	Name string // service or dependency
}

// WantedHostPorts returns the host ports the project would take on a free
// machine: declared ports as written, inferred ones at their default. It is
// the question "what does this project need?", asked without looking at
// what is bound right now — the allocator's usual bump-past-busy answer
// would hide exactly the collisions the caller wants to see.
func WantedHostPorts(deps *models.Deps) []WantedPort {
	detections := BuildDetectionMap(deps)
	free := func(string) (bool, error) { return false, nil }
	result, err := allocateHostPorts(deps, detections, nil, free)
	if err != nil {
		return nil
	}

	var out []WantedPort
	for name, alloc := range result.Services {
		if alloc.IsHost() && alloc.Port > 0 {
			out = append(out, WantedPort{Port: alloc.Port, Name: name})
		}
	}
	for name, alloc := range result.Deps {
		for _, m := range alloc.Mappings {
			if m.HostPort > 0 {
				out = append(out, WantedPort{Port: m.HostPort, Name: name})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Port != out[j].Port {
			return out[i].Port < out[j].Port
		}
		return out[i].Name < out[j].Name
	})
	return out
}
