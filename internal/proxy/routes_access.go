package proxy

import (
	"maps"
	"sort"

	"raioz/internal/domain/interfaces"
)

// sortedRoutes returns the routes in deterministic hostname order
// so generated Caddyfiles diff cleanly across runs. Issue 074
// closed the last unsorted producer; HostsLine already does the
// same for /etc/hosts output, and loadAllProjectRoutes sorts by
// project name. Iteration of a Go map is randomized, so the only
// stable shape is to sort here at the boundary between the map
// and any output that compares byte-for-byte.
func sortedRoutes(routes map[string]interfaces.ProxyRoute) []interfaces.ProxyRoute {
	out := make([]interfaces.ProxyRoute, 0, len(routes))
	for _, r := range routes {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Hostname < out[j].Hostname
	})
	return out
}

// snapshotRoutes returns a copy of the routes map so iterators
// release the RLock before doing slow work (docker exec, file
// write). ADR-028.
func (m *Manager) snapshotRoutes() map[string]interfaces.ProxyRoute {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make(map[string]interfaces.ProxyRoute, len(m.routes))
	maps.Copy(out, m.routes)
	return out
}

// routesCount is the lock-aware len(m.routes) for log fields.
func (m *Manager) routesCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.routes)
}
