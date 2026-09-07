package tunnel

import (
	"testing"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m.registryPath == "" {
		t.Error("expected non-empty registry path")
	}
}

func TestList_Empty(t *testing.T) {
	m := &Manager{registryPath: "/tmp/nonexistent-raioz-tunnels.json"}
	tunnels := m.List()
	if len(tunnels) != 0 {
		t.Errorf("expected 0, got %d", len(tunnels))
	}
}
