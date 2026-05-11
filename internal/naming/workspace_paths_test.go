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

	got := WorkspaceProxyDir()
	if !strings.HasSuffix(got, filepath.Join("acme", "proxy")) {
		t.Errorf("got %q, expected to end with acme/proxy", got)
	}
}

func TestWorkspaceCaddyfilePath(t *testing.T) {
	original := prefix
	defer func() { prefix = original }()
	prefix = "acme"

	got := WorkspaceCaddyfilePath()
	if !strings.HasSuffix(got, filepath.Join("acme", "proxy", "Caddyfile")) {
		t.Errorf("got %q, expected to end with acme/proxy/Caddyfile", got)
	}
}

// TestWorkspaceProxyDir_HonorsXDGStateHome locks in the issue 015 fix:
// proxy state must follow $XDG_STATE_HOME so reboots and Docker
// auto-create races can't poison it under /tmp.
func TestWorkspaceProxyDir_HonorsXDGStateHome(t *testing.T) {
	original := prefix
	defer func() { prefix = original }()
	prefix = "acme"

	xdg := t.TempDir()
	t.Setenv("XDG_STATE_HOME", xdg)

	got := WorkspaceProxyDir()
	want := filepath.Join(xdg, "acme", "proxy")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestWorkspaceProxyDir_FallsBackToHome covers the no-XDG case (most
// macOS / fresh Linux installs). Must land under ~/.local/state.
func TestWorkspaceProxyDir_FallsBackToHome(t *testing.T) {
	original := prefix
	defer func() { prefix = original }()
	prefix = "acme"

	t.Setenv("XDG_STATE_HOME", "")

	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		t.Skip("no home dir available on this runner")
	}

	got := WorkspaceProxyDir()
	want := filepath.Join(home, ".local", "state", "acme", "proxy")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
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

// TestProxyDir_HonorsXDGStateHome confirms the per-project legacy mode
// also moved out of /tmp. Same root cause, same fix.
func TestProxyDir_HonorsXDGStateHome(t *testing.T) {
	original := prefix
	defer func() { prefix = original }()
	prefix = "raioz"

	xdg := t.TempDir()
	t.Setenv("XDG_STATE_HOME", xdg)

	got := ProxyDir("billing")
	want := filepath.Join(xdg, "raioz-billing", "proxy")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
