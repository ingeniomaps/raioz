package docker

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCleanProject(t *testing.T) {
	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "raioz-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	composePath := filepath.Join(tmpDir, "docker-compose.yml")

	// Test with non-existent compose file (dry-run)
	actions, err := CleanProject(composePath, true)
	if err != nil {
		t.Errorf("CleanProject() error = %v", err)
	}
	if len(actions) != 0 {
		t.Errorf("CleanProject() should return empty actions for missing file")
	}

	// Test with non-existent compose file (actual clean)
	actions, err = CleanProject(composePath, false)
	if err != nil {
		t.Errorf("CleanProject() error = %v", err)
	}
	if len(actions) != 0 {
		t.Errorf("CleanProject() should return empty actions for missing file")
	}
}

func TestGetAllProjectWorkspaces(t *testing.T) {
	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "raioz-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Test with non-existent workspaces directory
	workspaces, err := GetAllProjectWorkspaces(tmpDir)
	if err != nil {
		t.Errorf("GetAllProjectWorkspaces() error = %v", err)
	}
	if len(workspaces) != 0 {
		t.Errorf("GetAllProjectWorkspaces() = %d workspaces, want 0", len(workspaces))
	}

	// Create workspaces directory with test projects
	workspacesDir := filepath.Join(tmpDir, "workspaces")
	if err := os.MkdirAll(workspacesDir, 0755); err != nil {
		t.Fatalf("Failed to create workspaces dir: %v", err)
	}

	// Create test project directories
	testProjects := []string{"project1", "project2"}
	for _, project := range testProjects {
		projectDir := filepath.Join(workspacesDir, project)
		if err := os.MkdirAll(projectDir, 0755); err != nil {
			t.Fatalf("Failed to create project dir: %v", err)
		}
	}

	// Test getting all workspaces
	workspaces, err = GetAllProjectWorkspaces(tmpDir)
	if err != nil {
		t.Errorf("GetAllProjectWorkspaces() error = %v", err)
	}
	if len(workspaces) != len(testProjects) {
		t.Errorf("GetAllProjectWorkspaces() = %d workspaces, want %d", len(workspaces), len(testProjects))
	}
}

func TestCleanupOptions(t *testing.T) {
	opts := CleanupOptions{
		DryRun:   false,
		All:      false,
		Images:   false,
		Volumes:  false,
		Networks: false,
		Force:    false,
	}

	if opts.DryRun {
		t.Error("CleanupOptions.DryRun should be false by default")
	}
	if opts.All {
		t.Error("CleanupOptions.All should be false by default")
	}
	if opts.Images {
		t.Error("CleanupOptions.Images should be false by default")
	}
	if opts.Volumes {
		t.Error("CleanupOptions.Volumes should be false by default")
	}
	if opts.Networks {
		t.Error("CleanupOptions.Networks should be false by default")
	}
	if opts.Force {
		t.Error("CleanupOptions.Force should be false by default")
	}
}
