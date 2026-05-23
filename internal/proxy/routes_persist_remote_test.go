package proxy

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"raioz/internal/domain/interfaces"
	"raioz/internal/naming"
)

// xdgIsolation redirects the XDG state base to a t.TempDir so the helper
// writes under a path the test owns. Restores on cleanup.
func xdgIsolation(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)
	return dir
}

func TestWriteRemoteProjectRoutes_WritesAtomicallyUnderWorkspace(t *testing.T) {
	xdg := xdgIsolation(t)
	route := interfaces.ProxyRoute{
		ServiceName: "api",
		Hostname:    "api",
		Target:      "https://api.staging.acme.dev",
	}
	err := WriteRemoteProjectRoutes("acme", "api", "localhost", "mkcert",
		[]interfaces.ProxyRoute{route})
	if err != nil {
		t.Fatalf("write: %v", err)
	}

	want := filepath.Join(xdg, "acme", "proxy", "routes", "api.json")
	_ = xdg
	data, err := os.ReadFile(want)
	if err != nil {
		t.Fatalf("read written file: %v", err)
	}

	var pp persistedProject
	if err := json.Unmarshal(data, &pp); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if pp.Project != "api" || pp.Domain != "localhost" || pp.TLSMode != "mkcert" {
		t.Errorf("envelope mismatch: %+v", pp)
	}
	if len(pp.Routes) != 1 || pp.Routes[0].Target != "https://api.staging.acme.dev" {
		t.Errorf("routes mismatch: %+v", pp.Routes)
	}
}

func TestWriteRemoteProjectRoutes_RejectsEmptyWorkspace(t *testing.T) {
	xdgIsolation(t)
	err := WriteRemoteProjectRoutes("", "api", "localhost", "mkcert", nil)
	if err == nil {
		t.Fatal("expected error for empty workspace")
	}
}

func TestWriteRemoteProjectRoutes_RejectsEmptyProject(t *testing.T) {
	xdgIsolation(t)
	err := WriteRemoteProjectRoutes("acme", "", "localhost", "mkcert", nil)
	if err == nil {
		t.Fatal("expected error for empty project")
	}
}

func TestWriteRemoteProjectRoutes_SanitizesProjectName(t *testing.T) {
	// Path-traversal guard: project names with separators or parent
	// refs get sanitized so the helper can't write outside the workspace
	// routes dir.
	xdg := xdgIsolation(t)
	err := WriteRemoteProjectRoutes("acme", "../escape", "localhost", "mkcert", nil)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	// File MUST live under the workspace's routes dir, NOT one level up.
	routesDir := filepath.Join(xdg, "acme", "proxy", "routes")
	entries, err := os.ReadDir(routesDir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("want 1 sanitized file, got %d entries: %v", len(entries), entries)
	}
	if entries[0].Name() == "../escape.json" {
		t.Errorf("filename leaked path traversal: %q", entries[0].Name())
	}
}

func TestWriteRemoteProjectRoutes_PicksWorkspaceProxyDir(t *testing.T) {
	xdg := xdgIsolation(t)
	if err := WriteRemoteProjectRoutes("acme", "api", "", "", nil); err != nil {
		t.Fatal(err)
	}
	// Pin the path the writer chose so callers in the meta runner can
	// trust it lines up with naming.WorkspaceProxyDirFor.
	want := filepath.Join(naming.WorkspaceProxyDirFor("acme"), "routes", "api.json")
	if _, err := os.Stat(want); err != nil {
		t.Errorf("expected file at %q: %v (xdg=%q)", want, err, xdg)
	}
}
