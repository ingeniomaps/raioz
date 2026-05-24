package proxy

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"raioz/internal/domain/interfaces"
	"raioz/internal/naming"
)

// makeSharedManager builds a Manager wired up for workspace-shared mode
// with the proxy state dir pointed at a fresh tempdir so each test runs
// in isolation. Both TMPDIR and XDG_STATE_HOME are redirected — XDG is
// what naming.WorkspaceProxyDir reads now that proxy state lives under
// $XDG_STATE_HOME; TMPDIR is kept for any code path that still touches os.TempDir
// (e.g. legacy helpers).
func makeSharedManager(t *testing.T, ws, project string) *Manager {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	t.Setenv("XDG_STATE_HOME", tmp)
	naming.SetPrefix(ws)
	t.Cleanup(func() { naming.SetPrefix("") })

	m := NewManager("")
	m.workspaceName = (ws)
	m.projectName = (project)
	m.domain = (ws + ".local")
	m.tlsMode = ("mkcert")
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
	m.projectName = ("solo") // no SetWorkspace
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
	m.projectName = ("beta")
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
	m.projectName = ("beta")
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
	if !strings.Contains(content, "auto_https disable_certs") {
		t.Errorf("expected auto_https disable_certs (mkcert mode):\n%s", content)
	}
}

// TestSaveProjectRoutes_AtomicUnderConcurrency stresses the atomic-write
// invariant in ADR-005: concurrent writers updating per-project files
// while many readers snapshot the union must never observe a partial or
// corrupt entry. Without the temp-file + rename used by
// SaveProjectRoutes, a reader can race a writer and read a half-written
// file that fails to unmarshal.
func TestSaveProjectRoutes_AtomicUnderConcurrency(t *testing.T) {
	if testing.Short() {
		t.Skip("concurrency stress test")
	}

	// Setup the workspace dir once via the shared helper so prefix/env
	// are configured. The seed manager itself isn't used for writes.
	_ = makeSharedManager(t, "wsConc", "seed")
	routesDir := filepath.Join(naming.WorkspaceProxyDir(), "routes")
	if err := os.MkdirAll(routesDir, 0o755); err != nil {
		t.Fatalf("mkdir routes dir: %v", err)
	}

	const writers = 10
	const readers = 10
	const opsPerGoroutine = 30

	writerManagers := make([]*Manager, writers)
	for i := range writerManagers {
		m := NewManager("")
		m.workspaceName = ("wsConc")
		m.projectName = (fmt.Sprintf("p%02d", i))
		m.domain = ("wsConc.local")
		m.tlsMode = ("mkcert")
		m.AddRoute(t.Context(), interfaces.ProxyRoute{
			ServiceName: "svc", Hostname: "svc",
			Target: fmt.Sprintf("p%02d-svc", i), Port: 3000,
		})
		writerManagers[i] = m
	}

	reader := NewManager("")
	reader.workspaceName = "wsConc"
	reader.projectName = "reader"
	reader.domain = "wsConc.local"
	reader.tlsMode = "mkcert"

	var wg sync.WaitGroup
	var corrupted atomic.Int64
	var saveErrors atomic.Int64

	for _, m := range writerManagers {
		wg.Go(func() {
			for range opsPerGoroutine {
				if err := m.SaveProjectRoutes(); err != nil {
					saveErrors.Add(1)
					return
				}
			}
		})
	}

	for range readers {
		wg.Go(func() {
			for range opsPerGoroutine {
				for _, pp := range reader.loadAllProjectRoutes() {
					// Atomic writes guarantee every observed file is
					// either the previous or new full version. Empty
					// Project or wrong Domain signals a partial read.
					if pp.Project == "" || pp.Domain != "wsConc.local" {
						corrupted.Add(1)
					}
				}
			}
		})
	}

	wg.Wait()

	if c := saveErrors.Load(); c != 0 {
		t.Errorf("%d save errors during concurrent writes", c)
	}
	if c := corrupted.Load(); c != 0 {
		t.Errorf("observed %d partial/corrupt loads under concurrent writes", c)
	}

	// After everything settles, every writer's project must be persisted.
	// Atomic rename guarantees no writer's last write is lost.
	seen := make(map[string]bool)
	for _, pp := range reader.loadAllProjectRoutes() {
		seen[pp.Project] = true
	}
	for i := range writers {
		want := fmt.Sprintf("p%02d", i)
		if !seen[want] {
			t.Errorf("project %s missing from final load (seen: %v)", want, seen)
		}
	}
}

func TestGenerateCaddyfile_PerProjectUsesInMemoryRoutes(t *testing.T) {
	naming.SetPrefix("")
	defer naming.SetPrefix("")

	m := NewManager("")
	m.projectName = ("solo")
	m.domain = ("solo.local")
	m.tlsMode = ("mkcert")
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
