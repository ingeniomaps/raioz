package docker

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestNetworkExists(t *testing.T) {
	// Test with non-existent network
	exists, info, err := NetworkExists("nonexistent-network-12345")
	if err != nil {
		t.Fatalf("NetworkExists() error = %v", err)
	}
	if exists {
		t.Error("NetworkExists() should return false for non-existent network")
	}
	if info != nil {
		t.Error("NetworkExists() should return nil info for non-existent network")
	}
}

func TestCreateNetwork(t *testing.T) {
	testNetwork := "raioz-test-network"

	// Cleanup in case it exists
	_ = RemoveNetwork(testNetwork)

	// Create network
	if err := CreateNetwork(testNetwork); err != nil {
		t.Fatalf("CreateNetwork() error = %v", err)
	}

	// Verify it exists
	exists, _, err := NetworkExists(testNetwork)
	if err != nil {
		t.Fatalf("NetworkExists() error = %v", err)
	}
	if !exists {
		t.Error("Network should exist after CreateNetwork()")
	}

	// Cleanup
	if err := RemoveNetwork(testNetwork); err != nil {
		t.Logf("Warning: failed to cleanup test network: %v", err)
	}
}

func TestEnsureNetwork(t *testing.T) {
	testNetwork := "raioz-test-ensure"

	// Cleanup in case it exists
	_ = RemoveNetwork(testNetwork)

	// First call should create network
	if err := EnsureNetwork(testNetwork); err != nil {
		t.Fatalf("EnsureNetwork() error = %v", err)
	}

	// Verify it exists
	exists, _, err := NetworkExists(testNetwork)
	if err != nil {
		t.Fatalf("NetworkExists() error = %v", err)
	}
	if !exists {
		t.Error("Network should exist after EnsureNetwork()")
	}

	// Second call should be idempotent (not error)
	if err := EnsureNetwork(testNetwork); err != nil {
		t.Fatalf("EnsureNetwork() should be idempotent, got error: %v", err)
	}

	// Cleanup
	if err := RemoveNetwork(testNetwork); err != nil {
		t.Logf("Warning: failed to cleanup test network: %v", err)
	}
}

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
		"project": map[string]any{
			"name":    "project1",
			"network": "test-network",
		},
	}
	state1Data, _ := json.Marshal(state1)
	os.WriteFile(filepath.Join(testProject1, ".state.json"), state1Data, 0644)

	// Create state file for project2 with different network
	state2 := map[string]any{
		"project": map[string]any{
			"name":    "project2",
			"network": "other-network",
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
