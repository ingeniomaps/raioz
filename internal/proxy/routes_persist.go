package proxy

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"raioz/internal/domain/interfaces"
	"raioz/internal/fsutil"
	"raioz/internal/logging"
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
//
// The write is atomic: data goes to a temp file in the same directory
// and is renamed onto the target path. A concurrent reader sees either
// the previous version or the new version, never a truncated mid-write.
// See ADR-005.
func (m *Manager) SaveProjectRoutes() error {
	path := m.projectRoutesPath()
	if path == "" {
		return nil
	}
	release, err := m.acquireWorkspaceLock()
	if err != nil {
		return err
	}
	defer release()

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create routes dir: %w", err)
	}

	// Sort routes by service name so the file content is stable across
	// runs (helpful for diff-friendly persistence and review). Snapshot
	// under RLock — workspace-shared mode can race AddRoute across
	// concurrent up flows. ADR-028.
	snap := m.snapshotRoutes()
	names := make([]string, 0, len(snap))
	for k := range snap {
		names = append(names, k)
	}
	sort.Strings(names)
	routes := make([]interfaces.ProxyRoute, 0, len(names))
	for _, n := range names {
		routes = append(routes, snap[n])
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

	tmp, err := os.CreateTemp(dir, ".routes-*.json.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpPath) }

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		cleanup()
		return fmt.Errorf("failed to write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return fmt.Errorf("failed to close temp file: %w", err)
	}
	if err := fsutil.RenameWithRetry(tmpPath, path); err != nil {
		cleanup()
		return fmt.Errorf("failed to rename routes file: %w", err)
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
	release, err := m.acquireWorkspaceLock()
	if err != nil {
		return err
	}
	defer release()

	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove routes file: %w", err)
	}
	return nil
}

// loadAllProjectRoutes reads every persisted project routes file in the
// workspace dir. Files that fail to read or parse are skipped with a
// warning — a corrupt single file shouldn't block the whole workspace
// from rendering, but neither should it disappear silently. Atomic
// writes in SaveProjectRoutes make parse errors a real signal that
// something external touched the file.
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
		full := filepath.Join(dir, e.Name())
		data, err := os.ReadFile(full)
		if err != nil {
			logging.Warn("failed to read project routes file, skipping",
				"file", e.Name(), "error", err)
			continue
		}
		var pp persistedProject
		if err := json.Unmarshal(data, &pp); err != nil {
			logging.Warn("corrupt project routes file, skipping",
				"file", e.Name(), "error", err)
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

// WriteRemoteProjectRoutes writes a persistedProject file for a meta
// sub-project that runs in remote mode (ADR-049). The meta runner has no
// Manager bound to it — sub-projects don't spawn locally — so this helper
// takes the workspace name explicitly and computes the routes directory
// the same way Manager.routesDir does for a workspace-shared manager.
//
// Pre-conditions:
//   - workspace MUST be non-empty (per-project mode has no shared dir).
//   - project MUST be safe-ish — the helper applies the same path-traversal
//     guard projectRoutesPath uses.
//   - tlsMode SHOULD be "mkcert" or "letsencrypt" — anything else falls
//     through to plain HTTP in the Caddyfile renderer.
//
// The on-disk shape matches what SaveProjectRoutes produces so the
// workspace Caddy reads remote and local routes through the same code
// path. Write is atomic (temp file + rename), matching ADR-005 invariant.
func WriteRemoteProjectRoutes(
	workspace, project, domain, tlsMode string,
	routes []interfaces.ProxyRoute,
) error {
	if workspace == "" {
		return fmt.Errorf("WriteRemoteProjectRoutes: workspace required")
	}
	if project == "" {
		return fmt.Errorf("WriteRemoteProjectRoutes: project required")
	}

	dir := workspaceRoutesDirFor(workspace)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create routes dir: %w", err)
	}
	path := filepath.Join(dir, safeRoutesFilename(project))

	pp := persistedProject{
		Project: project,
		Domain:  domain,
		TLSMode: tlsMode,
		Routes:  append([]interfaces.ProxyRoute(nil), routes...),
	}
	data, err := json.MarshalIndent(pp, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal remote routes: %w", err)
	}

	tmp, err := os.CreateTemp(dir, ".routes-*.json.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpPath) }
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		cleanup()
		return fmt.Errorf("failed to write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return fmt.Errorf("failed to close temp file: %w", err)
	}
	if err := fsutil.RenameWithRetry(tmpPath, path); err != nil {
		cleanup()
		return fmt.Errorf("failed to rename routes file: %w", err)
	}
	return nil
}

// workspaceRoutesDirFor is the standalone equivalent of Manager.routesDir,
// taking the workspace name as input rather than reading it off Manager
// state. Necessary because WriteRemoteProjectRoutes runs from contexts
// (the meta runner) that don't instantiate a Manager.
func workspaceRoutesDirFor(workspace string) string {
	return filepath.Join(naming.WorkspaceProxyDirFor(workspace), "routes")
}

// safeRoutesFilename mirrors the projectRoutesPath guard against path
// separators / parent traversal. Both `/` and `\` are stripped regardless
// of the host OS — a project name containing a foreign separator must
// not let the file escape the workspace routes dir.
func safeRoutesFilename(project string) string {
	safe := strings.ReplaceAll(project, "/", "_")
	safe = strings.ReplaceAll(safe, "\\", "_")
	safe = strings.ReplaceAll(safe, "..", "_")
	return safe + ".json"
}
