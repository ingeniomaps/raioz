package proxy

import (
	"maps"

	"raioz/internal/domain/interfaces"
)

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
