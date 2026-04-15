package proxy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"raioz/internal/domain/interfaces"
	"raioz/internal/naming"
)

// makeSharedManager builds a Manager wired up for workspace-shared mode
// with HOME/RAIOZ_HOME/temp pointed at a fresh tempdir so each test runs
// in isolation.
func makeSharedManager(t *testing.T, ws, project string) *Manager {
	t.Helper()
	t.Setenv("TMPDIR", t.TempDir())
	naming.SetPrefix(ws)
	t.Cleanup(func() { naming.SetPrefix("") })

	m := NewManager("")
	m.SetWorkspace(ws)
	m.SetProjectName(project)
	m.SetDomain(ws + ".local")
	m.SetTLSMode("mkcert")
	return m
}

func TestSaveProjectRoutes_PersistsToWorkspaceDir(t *testing.T) {
	m := makeSharedManager(t, "wsA", "alpha")
	m.AddRoute(t.Context(), interfaces.ProxyRoute{
		ServiceName: "api", Hostname: "api", Target: "alpha-api", Port: 3000,
	})

	if err := m.SaveProjectRoutes(); err != nil {
		t.Fatalf("save: %v", err)
	}
	want := filepath.Join(naming.WorkspaceProxyDir(), "routes", "alpha.json")
	data, err := os.ReadFile(want)
	if err != nil {
		t.Fatalf("expected file at %s, got %v", want, err)
	}
	body := string(data)
	if !strings.Contains(body, `"project": "alpha"`) ||
		!strings.Contains(body, `"domain": "wsA.local"`) ||
		!strings.Contains(body, `"ServiceName": "api"`) {
		t.Errorf("file content missing expected fields: %s", body)
	}
}

func TestSaveProjectRoutes_NoOpOutsideWorkspace(t *testing.T) {
	m := NewManager("")
	m.SetProjectName("solo") // no SetWorkspace
	if err := m.SaveProjectRoutes(); err != nil {
		t.Errorf("per-project mode must be a no-op, got %v", err)
	}
}

func TestRemoveProjectRoutes_DeletesFile(t *testing.T) {
	m := makeSharedManager(t, "wsB", "beta")
	m.AddRoute(t.Context(), interfaces.ProxyRoute{ServiceName: "x", Hostname: "x"})
	if err := m.SaveProjectRoutes(); err != nil {
		t.Fatal(err)
	}
	path := m.projectRoutesPath()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected pre-existing routes file, got %v", err)
	}
	if err := m.RemoveProjectRoutes(); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("routes file must be gone after remove, stat err=%v", err)
	}
}

func TestRemoveProjectRoutes_IdempotentOnMissing(t *testing.T) {
	m := makeSharedManager(t, "wsC", "gamma")
	if err := m.RemoveProjectRoutes(); err != nil {
		t.Errorf("removing a non-existent file must succeed silently, got %v", err)
	}
}

func TestRemainingProjects_CountsFiles(t *testing.T) {
	m := makeSharedManager(t, "wsD", "alpha")
	if got := m.RemainingProjects(); got != 0 {
		t.Errorf("empty dir → 0 remaining, got %d", got)
	}
	m.AddRoute(t.Context(), interfaces.ProxyRoute{ServiceName: "a", Hostname: "a"})
	_ = m.SaveProjectRoutes()
	if got := m.RemainingProjects(); got != 1 {
		t.Errorf("one project → 1 remaining, got %d", got)
	}

	// Drop a sibling file by switching project name.
	m.SetProjectName("beta")
	m.AddRoute(t.Context(), interfaces.ProxyRoute{ServiceName: "b", Hostname: "b"})
	_ = m.SaveProjectRoutes()
	if got := m.RemainingProjects(); got != 2 {
		t.Errorf("two projects → 2 remaining, got %d", got)
	}
}

func TestGenerateCaddyfile_SharedMergesAcrossProjects(t *testing.T) {
	m := makeSharedManager(t, "wsE", "alpha")

	// Project alpha persists its routes.
	m.AddRoute(t.Context(), interfaces.ProxyRoute{
		ServiceName: "api", Hostname: "api", Target: "alpha-api", Port: 3000,
	})
	if err := m.SaveProjectRoutes(); err != nil {
		t.Fatal(err)
	}

	// Switch to project beta and persist a different route.
	m.SetProjectName("beta")
	// Reset in-memory routes since AddRoute keeps appending across projects.
	m.routes = map[string]interfaces.ProxyRoute{}
	m.AddRoute(t.Context(), interfaces.ProxyRoute{
		ServiceName: "web", Hostname: "web", Target: "beta-web", Port: 8080,
	})
	if err := m.SaveProjectRoutes(); err != nil {
		t.Fatal(err)
	}

	// generateCaddyfile must include BOTH projects' routes — that's the
	// whole point of Phase C. Without the union, beta's up would erase
	// alpha's HTTPS routing.
	path, err := m.generateCaddyfile()
	if err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(path)
	content := string(body)
	if !strings.Contains(content, "alpha-api:3000") {
		t.Errorf("alpha's route missing from shared Caddyfile:\n%s", content)
	}
	if !strings.Contains(content, "beta-web:8080") {
		t.Errorf("beta's route missing from shared Caddyfile:\n%s", content)
	}
	if !strings.Contains(content, "auto_https off") {
		t.Errorf("expected auto_https off (mkcert mode):\n%s", content)
	}
}

func TestGenerateCaddyfile_PerProjectUsesInMemoryRoutes(t *testing.T) {
	naming.SetPrefix("")
	defer naming.SetPrefix("")

	m := NewManager("")
	m.SetProjectName("solo")
	m.SetDomain("solo.local")
	m.SetTLSMode("mkcert")
	m.networkName = "solo-net" // ProxyDir uses this
	t.Setenv("TMPDIR", t.TempDir())

	m.AddRoute(t.Context(), interfaces.ProxyRoute{
		ServiceName: "api", Hostname: "api", Target: "solo-api", Port: 3000,
	})

	path, err := m.generateCaddyfile()
	if err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(path)
	if !strings.Contains(string(body), "solo-api:3000") {
		t.Errorf("per-project mode must render m.routes, got:\n%s", body)
	}
}
