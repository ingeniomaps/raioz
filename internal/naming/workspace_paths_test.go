package naming

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkspaceProxyContainer(t *testing.T) {
	original := prefix
	defer func() { prefix = original }()
	prefix = "acme"

	if got := WorkspaceProxyContainer(); got != "acme-proxy" {
		t.Errorf("got %q, want acme-proxy", got)
	}
}

func TestWorkspaceCaddyVolume(t *testing.T) {
	original := prefix
	defer func() { prefix = original }()
	prefix = "acme"

	if got := WorkspaceCaddyVolume(); got != "acme-caddy" {
		t.Errorf("got %q, want acme-caddy", got)
	}
}

func TestWorkspaceProxyDir(t *testing.T) {
	original := prefix
	defer func() { prefix = original }()
	prefix = "acme"
	t.Setenv("RAIOZ_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	got := WorkspaceProxyDir()
	if !strings.HasSuffix(got, filepath.Join("proxies", "acme")) {
		t.Errorf("got %q, expected to end with proxies/acme", got)
	}
}

// TestWorkspaceProxyDir_UnderStateDir locks the location in: proxy state
// lives inside the raioz state dir (ADR-022), never beside it and never
// under /tmp, where a reboot or a Docker auto-create race can poison it.
func TestWorkspaceProxyDir_UnderStateDir(t *testing.T) {
	original := prefix
	defer func() { prefix = original }()
	prefix = "acme"

	xdg := t.TempDir()
	t.Setenv("RAIOZ_HOME", "")
	t.Setenv("XDG_STATE_HOME", xdg)

	got := WorkspaceProxyDir()
	want := filepath.Join(xdg, "raioz", "proxies", "acme")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	home := t.TempDir()
	t.Setenv("RAIOZ_HOME", home)
	if got, want := WorkspaceProxyDir(), filepath.Join(home, "proxies", "acme"); got != want {
		t.Errorf("with RAIOZ_HOME: got %q, want %q", got, want)
	}
}

// TestProxyDir_KeepsLegacyWhilePresent: the directory an older raioz used
// is the bind-mount source of a proxy that may still be running, so it
// stays the answer until the `down` that stops that proxy removes it.
func TestProxyDir_KeepsLegacyWhilePresent(t *testing.T) {
	original := prefix
	defer func() { prefix = original }()
	prefix = "acme"

	xdg := t.TempDir()
	t.Setenv("RAIOZ_HOME", "")
	t.Setenv("XDG_STATE_HOME", xdg)

	legacy := filepath.Join(xdg, "acme", "proxy")
	if err := os.MkdirAll(legacy, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := WorkspaceProxyDir(); got != legacy {
		t.Errorf("legacy dir present: got %q, want %q", got, legacy)
	}

	if err := os.RemoveAll(legacy); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(xdg, "raioz", "proxies", "acme")
	if got := WorkspaceProxyDir(); got != want {
		t.Errorf("legacy dir gone: got %q, want %q", got, want)
	}
}

// TestLegacyWorkspaceProxyDir checks the back-compat helper still points
// at the pre-XDG /tmp location. cleanProxyDirOnDisk relies on this to
// migrate legacy installs on the next down.
func TestLegacyWorkspaceProxyDir(t *testing.T) {
	original := prefix
	defer func() { prefix = original }()
	prefix = "acme"

	got := LegacyWorkspaceProxyDir()
	want := filepath.Join(os.TempDir(), "acme", "proxy")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestLegacyProxyDir mirrors LegacyWorkspaceProxyDir for the per-project
// (non-workspace) lifecycle.
func TestLegacyProxyDir(t *testing.T) {
	original := prefix
	defer func() { prefix = original }()
	prefix = "raioz"

	got := LegacyProxyDir("billing")
	want := filepath.Join(os.TempDir(), "raioz-billing", "proxy")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestProxyDir_UnderStateDir is the per-project (non-workspace) twin of
// TestWorkspaceProxyDir_UnderStateDir.
func TestProxyDir_UnderStateDir(t *testing.T) {
	original := prefix
	defer func() { prefix = original }()
	prefix = "raioz"

	xdg := t.TempDir()
	t.Setenv("RAIOZ_HOME", "")
	t.Setenv("XDG_STATE_HOME", xdg)

	got := ProxyDir("billing")
	want := filepath.Join(xdg, "raioz", "proxies", "raioz-billing")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
