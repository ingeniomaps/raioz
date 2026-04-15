package naming

import (
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
