package snapshot

import (
	"testing"
)

func TestNewManager(t *testing.T) {
	m := NewManager("/tmp/test-snapshots")
	if m.baseDir != "/tmp/test-snapshots" {
		t.Errorf("expected /tmp/test-snapshots, got %s", m.baseDir)
	}
}

func TestNewManager_Default(t *testing.T) {
	m := NewManager("")
	if m.baseDir == "" {
		t.Error("expected non-empty default baseDir")
	}
}

func TestList_EmptyProject(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	snapshots, err := m.List("nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(snapshots) != 0 {
		t.Errorf("expected 0 snapshots, got %d", len(snapshots))
	}
}

func TestDelete_Nonexistent(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	// Should not error on nonexistent snapshot
	err := m.Delete("project", "nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRestore_Nonexistent(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	err := m.Restore("project", "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent snapshot")
	}
}
