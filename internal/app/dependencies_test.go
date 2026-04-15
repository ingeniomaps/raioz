package app

import (
	"testing"
)

func TestNewDependencies(t *testing.T) {
	deps := NewDependencies()
	if deps == nil {
		t.Fatal("expected non-nil Dependencies")
	}
	if deps.ConfigLoader == nil {
		t.Error("expected ConfigLoader to be set")
	}
	if deps.Validator == nil {
		t.Error("expected Validator to be set")
	}
	if deps.DockerRunner == nil {
		t.Error("expected DockerRunner to be set")
	}
	if deps.GitRepository == nil {
		t.Error("expected GitRepository to be set")
	}
	if deps.Workspace == nil {
		t.Error("expected Workspace to be set")
	}
	if deps.StateManager == nil {
		t.Error("expected StateManager to be set")
	}
	if deps.LockManager == nil {
		t.Error("expected LockManager to be set")
	}
	if deps.HostRunner == nil {
		t.Error("expected HostRunner to be set")
	}
	if deps.EnvManager == nil {
		t.Error("expected EnvManager to be set")
	}
	if deps.ProxyManager == nil {
		t.Error("expected ProxyManager to be set")
	}
	if deps.DiscoveryManager == nil {
		t.Error("expected DiscoveryManager to be set")
	}
}
