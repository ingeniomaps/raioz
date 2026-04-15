package proxy

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"raioz/internal/domain/interfaces"
	"raioz/internal/naming"
)

// persistedProject is the on-disk shape of a single project's contribution
// to the workspace-shared Caddyfile. Each project drops one of these into
// the workspace routes directory at `up` time and removes it at `down`.
// The proxy reads the union to render its config.
type persistedProject struct {
	Project string                  `json:"project"`
	Domain  string                  `json:"domain"`
	TLSMode string                  `json:"tlsMode"`
	Routes  []interfaces.ProxyRoute `json:"routes"`
}

// routesDir is the directory under the workspace's proxy temp tree where
// per-project route files live. Returns "" when no workspace is active —
// callers must check before reading/writing.
func (m *Manager) routesDir() string {
	if !m.isWorkspaceShared() {
		return ""
	}
	return filepath.Join(naming.WorkspaceProxyDir(), "routes")
}

// projectRoutesPath returns the absolute path to this project's persisted
// routes file.
func (m *Manager) projectRoutesPath() string {
	dir := m.routesDir()
	if dir == "" || m.projectName == "" {
		return ""
	}
	// The project name is part of a filename, so guard against path
	// traversal even though raioz.yaml validation should already block it.
	safe := strings.ReplaceAll(m.projectName, string(filepath.Separator), "_")
	safe = strings.ReplaceAll(safe, "..", "_")
	return filepath.Join(dir, safe+".json")
}

// SaveProjectRoutes writes the manager's in-memory routes (plus the
// project's domain and tlsMode) to disk so a later workspace-shared
// Caddyfile generation can include them. Per-project mode is a no-op.
func (m *Manager) SaveProjectRoutes() error {
	path := m.projectRoutesPath()
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("failed to create routes dir: %w", err)
	}

	// Sort routes by service name so the file content is stable across
	// runs (helpful for diff-friendly persistence and review).
	names := make([]string, 0, len(m.routes))
	for k := range m.routes {
		names = append(names, k)
	}
	sort.Strings(names)
	routes := make([]interfaces.ProxyRoute, 0, len(names))
	for _, n := range names {
		routes = append(routes, m.routes[n])
	}

	pp := persistedProject{
		Project: m.projectName,
		Domain:  m.domain,
		TLSMode: m.tlsMode,
		Routes:  routes,
	}
	data, err := json.MarshalIndent(pp, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal routes: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("failed to write routes file: %w", err)
	}
	return nil
}

// RemoveProjectRoutes deletes this project's persisted routes file. Idempotent
// (missing file is not an error) — the goal is "after this returns, the file
// is gone", not "the file existed".
func (m *Manager) RemoveProjectRoutes() error {
	path := m.projectRoutesPath()
	if path == "" {
		return nil
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove routes file: %w", err)
	}
	return nil
}

// loadAllProjectRoutes reads every persisted project routes file in the
// workspace dir. Files that fail to parse are skipped with a logged hint
// rather than failing the whole load — a corrupt single file shouldn't
// block the whole workspace from rendering.
func (m *Manager) loadAllProjectRoutes() []persistedProject {
	dir := m.routesDir()
	if dir == "" {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var all []persistedProject
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var pp persistedProject
		if err := json.Unmarshal(data, &pp); err != nil {
			continue
		}
		all = append(all, pp)
	}

	// Stable order (project name) so generated Caddyfiles diff cleanly.
	sort.Slice(all, func(i, j int) bool {
		return all[i].Project < all[j].Project
	})
	return all
}

// RemainingProjects returns the count of persisted project routes files in
// the workspace dir. Callers use it to decide between Reload (others
// remain) and Stop (last one out turns off the lights).
func (m *Manager) RemainingProjects() int {
	return len(m.loadAllProjectRoutes())
}
