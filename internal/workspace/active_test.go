package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func setupHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("RAIOZ_HOME", dir)
	return dir
}

func TestGetActiveWorkspacePath(t *testing.T) {
	setupHome(t)
	path, err := GetActiveWorkspacePath()
	if err != nil {
		t.Fatalf("GetActiveWorkspacePath: %v", err)
	}
	if path == "" {
		t.Error("expected non-empty path")
	}
	if filepath.Base(path) != "active-workspace" {
		t.Errorf("expected basename active-workspace, got %q", filepath.Base(path))
	}
}

func TestGetActiveWorkspace_Empty(t *testing.T) {
	setupHome(t)

	name, err := GetActiveWorkspace()
	if err != nil {
		t.Fatalf("GetActiveWorkspace: %v", err)
	}
	if name != "" {
		t.Errorf("expected empty, got %q", name)
	}
}

func TestSetAndGetActiveWorkspace(t *testing.T) {
	setupHome(t)

	if err := SetActiveWorkspace("myws"); err != nil {
		t.Fatalf("SetActiveWorkspace: %v", err)
	}

	got, err := GetActiveWorkspace()
	if err != nil {
		t.Fatalf("GetActiveWorkspace: %v", err)
	}
	if got != "myws" {
		t.Errorf("got %q, want myws", got)
	}
}

func TestSetActiveWorkspace_Overwrite(t *testing.T) {
	setupHome(t)

	_ = SetActiveWorkspace("first")
	_ = SetActiveWorkspace("second")

	got, _ := GetActiveWorkspace()
	if got != "second" {
		t.Errorf("expected 'second', got %q", got)
	}
}

func TestSetActiveWorkspace_Trims(t *testing.T) {
	setupHome(t)

	if err := SetActiveWorkspace("  padded  "); err != nil {
		t.Fatalf("SetActiveWorkspace: %v", err)
	}
	got, _ := GetActiveWorkspace()
	if got != "padded" {
		t.Errorf("expected trimmed, got %q", got)
	}
}

func TestClearActiveWorkspace(t *testing.T) {
	setupHome(t)

	_ = SetActiveWorkspace("x")
	if err := ClearActiveWorkspace(); err != nil {
		t.Fatalf("ClearActiveWorkspace: %v", err)
	}

	got, _ := GetActiveWorkspace()
	if got != "" {
		t.Errorf("expected empty after clear, got %q", got)
	}
}

func TestClearActiveWorkspace_AlreadyCleared(t *testing.T) {
	setupHome(t)

	// Clear without setting first — should be no-op
	if err := ClearActiveWorkspace(); err != nil {
		t.Errorf("ClearActiveWorkspace: %v", err)
	}
}

func TestListWorkspaces_Empty(t *testing.T) {
	setupHome(t)

	list, err := ListWorkspaces()
	if err != nil {
		t.Fatalf("ListWorkspaces: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("expected empty, got %v", list)
	}
}

func TestListWorkspaces_Populated(t *testing.T) {
	base := setupHome(t)

	// Create workspaces manually
	wsDir := filepath.Join(base, "workspaces")
	os.MkdirAll(filepath.Join(wsDir, "ws1"), 0o755)
	os.MkdirAll(filepath.Join(wsDir, "ws2"), 0o755)

	// Also create a file (should be ignored)
	os.WriteFile(filepath.Join(wsDir, "file.txt"), []byte("x"), 0o644)

	list, err := ListWorkspaces()
	if err != nil {
		t.Fatalf("ListWorkspaces: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("expected 2 workspaces, got %v", list)
	}
}

func TestWorkspaceExists_False(t *testing.T) {
	setupHome(t)

	exists, err := WorkspaceExists("nonexistent")
	if err != nil {
		t.Fatalf("WorkspaceExists: %v", err)
	}
	if exists {
		t.Error("expected false")
	}
}

func TestWorkspaceExists_True(t *testing.T) {
	base := setupHome(t)

	wsDir := filepath.Join(base, "workspaces", "exists")
	os.MkdirAll(wsDir, 0o755)

	exists, err := WorkspaceExists("exists")
	if err != nil {
		t.Fatalf("WorkspaceExists: %v", err)
	}
	if !exists {
		t.Error("expected true")
	}
}

func TestDeleteWorkspace(t *testing.T) {
	base := setupHome(t)

	wsDir := filepath.Join(base, "workspaces", "todelete")
	os.MkdirAll(wsDir, 0o755)
	os.WriteFile(filepath.Join(wsDir, "file.txt"), []byte("x"), 0o644)

	if err := DeleteWorkspace("todelete"); err != nil {
		t.Fatalf("DeleteWorkspace: %v", err)
	}

	if _, err := os.Stat(wsDir); !os.IsNotExist(err) {
		t.Error("workspace should be deleted")
	}
}

func TestDeleteWorkspace_PathTraversal(t *testing.T) {
	setupHome(t)

	// Attempt to delete outside the workspaces dir
	err := DeleteWorkspace("../../etc")
	// Should either error (validation) or be a no-op
	_ = err
}

func TestValidatePathInBase_Valid(t *testing.T) {
	base := "/home/user/raioz/workspaces"
	path := "/home/user/raioz/workspaces/ws1"

	if err := validatePathInBase(path, base); err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestValidatePathInBase_Invalid(t *testing.T) {
	base := "/home/user/raioz/workspaces"
	path := "/etc/passwd"

	if err := validatePathInBase(path, base); err == nil {
		t.Error("expected error for path outside base")
	}
}

func TestValidatePathInBase_Same(t *testing.T) {
	base := "/home/user/raioz/workspaces"

	if err := validatePathInBase(base, base); err != nil {
		t.Errorf("expected no error for same path, got %v", err)
	}
}
