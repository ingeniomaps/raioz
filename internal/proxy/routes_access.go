package proxy

import (
	"maps"
	"sort"
	"strings"

	"raioz/internal/domain/interfaces"
)

// sortedRoutes returns the routes in hostname order so generated
// Caddyfiles diff cleanly across runs (map iteration is randomized).
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

// HostsLine renders an /etc/hosts entry that maps every route the manager
// knows about to the proxy's container IP. Returns "" when no IP is
// resolvable or when there are no routes — both signal "nothing useful to
// print" rather than an error condition.
func (m *Manager) HostsLine() string {
	ip := m.ContainerIP()
	routes := m.snapshotRoutes()
	if ip == "" || len(routes) == 0 {
		return ""
	}
	hosts := make([]string, 0, len(routes))
	for _, route := range routes {
		hosts = append(hosts, route.Hostname+"."+m.domain)
	}
	sort.Strings(hosts) // stable output for diffs / docs
	return ip + "  " + strings.Join(hosts, " ")
}

// routeSANs returns the exact FQDNs of every route the manager knows about
// under its own domain, deduplicated, for use as additional mkcert SANs. An
// apex hostname like conorbi.localhost is only matched by the *.localhost
// wildcard, which browsers reject because the parent is a single label — so
// the FQDN must be minted explicitly. In workspace-shared mode the cert is
// per-domain, so we fold in every sibling project sharing this domain.
// See ADR-003.
func (m *Manager) routeSANs() []string {
	seen := make(map[string]bool)
	var sans []string
	add := func(host string) {
		if host == "" || seen[host] {
			return
		}
		seen[host] = true
		sans = append(sans, host)
	}
	if m.isWorkspaceShared() {
		for _, pp := range m.loadAllProjectRoutes() {
			if pp.Domain != m.domain {
				continue
			}
			for _, r := range pp.Routes {
				for _, h := range routeHostnames(r, pp.Domain) {
					add(h)
				}
			}
		}
	}
	for _, r := range m.snapshotRoutes() {
		for _, h := range routeHostnames(r, m.domain) {
			add(h)
		}
	}
	return sans
}
