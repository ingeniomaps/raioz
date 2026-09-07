package docker

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestGetNetworkProjects(t *testing.T) {
	tmpDir := t.TempDir()
	workspacesDir := filepath.Join(tmpDir, "workspaces")

	// Create test workspaces
	testProject1 := filepath.Join(workspacesDir, "project1")
	testProject2 := filepath.Join(workspacesDir, "project2")

	if err := os.MkdirAll(testProject1, 0755); err != nil {
		t.Fatalf("Failed to create test project1: %v", err)
	}
	if err := os.MkdirAll(testProject2, 0755); err != nil {
		t.Fatalf("Failed to create test project2: %v", err)
	}

	// Create state file for project1 with network "test-network"
	state1 := map[string]any{
		"network": "test-network",
		"project": map[string]any{
			"name": "project1",
		},
	}
	state1Data, _ := json.Marshal(state1)
	os.WriteFile(filepath.Join(testProject1, ".state.json"), state1Data, 0644)

	// Create state file for project2 with different network
	state2 := map[string]any{
		"network": "other-network",
		"project": map[string]any{
			"name": "project2",
		},
	}
	state2Data, _ := json.Marshal(state2)
	os.WriteFile(filepath.Join(testProject2, ".state.json"), state2Data, 0644)

	// Test finding projects using "test-network"
	projects, err := GetNetworkProjects("test-network", tmpDir)
	if err != nil {
		t.Fatalf("GetNetworkProjects() error = %v", err)
	}

	if len(projects) != 1 {
		t.Errorf("GetNetworkProjects() found %d projects, want 1", len(projects))
	}

	if len(projects) > 0 && projects[0] != "project1" {
		t.Errorf("GetNetworkProjects() found project %s, want project1", projects[0])
	}
}
