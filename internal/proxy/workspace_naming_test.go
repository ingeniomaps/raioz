package proxy

import (
	"testing"

	"raioz/internal/naming"
)

func TestManager_ContainerName_PerProject(t *testing.T) {
	naming.SetPrefix("")
	defer naming.SetPrefix("")

	m := NewManager("")
	m.SetProjectName("api")
	// no workspace → legacy per-project naming
	got := m.containerName()
	if got != "raioz-proxy-api" {
		t.Errorf("expected raioz-proxy-api, got %q", got)
	}
}

func TestManager_ContainerName_WorkspaceShared(t *testing.T) {
	naming.SetPrefix("acme")
	defer naming.SetPrefix("")

	m := NewManager("")
	m.SetProjectName("api")
	m.SetWorkspace("acme")

	got := m.containerName()
	if got != "acme-proxy" {
		t.Errorf("workspace-shared name should be {workspace}-proxy, got %q", got)
	}
}

func TestManager_CaddyVolume_PerProject(t *testing.T) {
	naming.SetPrefix("")
	defer naming.SetPrefix("")

	m := NewManager("")
	m.SetProjectName("api")
	got := m.caddyVolume()
	if got != "raioz-caddy-api" {
		t.Errorf("expected raioz-caddy-api, got %q", got)
	}
}

func TestManager_CaddyVolume_WorkspaceShared(t *testing.T) {
	naming.SetPrefix("acme")
	defer naming.SetPrefix("")

	m := NewManager("")
	m.SetProjectName("api")
	m.SetWorkspace("acme")

	got := m.caddyVolume()
	if got != "acme-caddy" {
		t.Errorf("workspace volume should be {workspace}-caddy, got %q", got)
	}
}

func TestManager_IsWorkspaceShared(t *testing.T) {
	m := NewManager("")
	if m.isWorkspaceShared() {
		t.Error("default Manager should not be in workspace-shared mode")
	}
	m.SetWorkspace("acme")
	if !m.isWorkspaceShared() {
		t.Error("after SetWorkspace(non-empty), must be shared")
	}
	m.SetWorkspace("")
	if m.isWorkspaceShared() {
		t.Error("SetWorkspace(\"\") must revert to per-project mode")
	}
}

func TestNamingHelpers_WorkspaceProxy(t *testing.T) {
	naming.SetPrefix("acme")
	defer naming.SetPrefix("")

	if naming.WorkspaceProxyContainer() != "acme-proxy" {
		t.Errorf("WorkspaceProxyContainer = %q, want acme-proxy", naming.WorkspaceProxyContainer())
	}
	if naming.WorkspaceCaddyVolume() != "acme-caddy" {
		t.Errorf("WorkspaceCaddyVolume = %q, want acme-caddy", naming.WorkspaceCaddyVolume())
	}
	dir := naming.WorkspaceProxyDir()
	if dir == "" {
		t.Error("WorkspaceProxyDir should be non-empty")
	}
}
