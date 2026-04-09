package tunnel

import (
	"testing"
)

func TestDetectBackend(t *testing.T) {
	// Just verify it doesn't panic
	_, _ = DetectBackend()
}

func TestHasBackend(t *testing.T) {
	_ = HasBackend()
}

func TestFormatURL(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"https://abc123.trycloudflare.com", "abc123.trycloudflare.com"},
		{"bore.pub:12345", "bore.pub:12345"},
	}
	for _, tt := range tests {
		if got := FormatURL(tt.input); got != tt.want {
			t.Errorf("FormatURL(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

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
