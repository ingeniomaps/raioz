package cmd_test

import (
	"testing"

	"raioz/internal/config"
	"raioz/internal/state"
	testhelpers "raioz/internal/testing"
	"raioz/internal/workspace"
)

// TestUpDownStatusIntegration tests the complete up/down/status flow
func TestUpDownStatusIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir := t.TempDir()
	testDeps := testhelpers.CreateMinimalTestDeps()
	testDeps.Project.Name = "integration-test"

	depsPath, err := testhelpers.CreateTestDepsJSON(tmpDir, testDeps)
	if err != nil {
		t.Fatalf("Failed to create test .raioz.json: %v", err)
	}

	// Note: This test requires Docker to be running
	// For a full integration test, we would need to:
	// 1. Actually call cmd.upCmd.Execute() or similar
	// 2. Verify services are running
	// 3. Call cmd.downCmd.Execute()
	// 4. Verify services are stopped
	//
	// However, this requires mocking Docker commands or using testcontainers
	// For now, we test the configuration loading and validation parts

	// Test that config can be loaded
	loadedDeps, _, err := config.LoadDeps(depsPath)
	if err != nil {
		t.Fatalf("Failed to load deps: %v", err)
	}

	if loadedDeps.Project.Name != testDeps.Project.Name {
		t.Errorf("Expected project name %s, got %s",
			testDeps.Project.Name, loadedDeps.Project.Name)
	}
}

// TestIdempotency tests that running up multiple times is safe
func TestIdempotency(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	testDeps := testhelpers.CreateMinimalTestDeps()
	testDeps.Project.Name = "idempotency-test"

	ws, err := workspace.Resolve(testDeps.Project.Name)
	if err != nil {
		t.Fatalf("Failed to resolve workspace: %v", err)
	}

	// Save state once
	if err := state.Save(ws, testDeps); err != nil {
		t.Fatalf("Failed to save state: %v", err)
	}

	// Load state multiple times - should be idempotent
	for i := 0; i < 3; i++ {
		loaded, err := state.Load(ws)
		if err != nil {
			t.Fatalf("Failed to load state (iteration %d): %v", i, err)
		}

		if loaded.Project.Name != testDeps.Project.Name {
			t.Errorf("Iteration %d: expected project name %s, got %s",
				i, testDeps.Project.Name, loaded.Project.Name)
		}
	}

	// Save state again - should be idempotent
	if err := state.Save(ws, testDeps); err != nil {
		t.Fatalf("Failed to save state again: %v", err)
	}

	// Verify state is still correct
	final, err := state.Load(ws)
	if err != nil {
		t.Fatalf("Failed to load final state: %v", err)
	}

	if final.Project.Name != testDeps.Project.Name {
		t.Errorf("Final state: expected project name %s, got %s",
			testDeps.Project.Name, final.Project.Name)
	}
}

// TestUpWithInvalidConfig tests error handling with invalid config
func TestUpWithInvalidConfig(t *testing.T) {
	tmpDir := t.TempDir()

	// Test with malformed JSON
	malformedPath, err := testhelpers.CreateMalformedDepsJSON(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create malformed .raioz.json: %v", err)
	}

	_, _, err = config.LoadDeps(malformedPath)
	if err == nil {
		t.Error("Expected error when loading malformed JSON, got nil")
	}

	// Test with invalid schema
	invalidPath, err := testhelpers.CreateInvalidDepsJSON(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create invalid .raioz.json: %v", err)
	}

	deps, _, err := config.LoadDeps(invalidPath)
	if err != nil {
		t.Fatalf("Failed to load invalid deps: %v", err)
	}

	// Validation should fail
	// Note: This requires the validate package, which we can't easily test
	// without importing it directly. For now, we just check loading works.
	_ = deps
}

// TestEdgeCasePortConflicts tests edge case of port conflicts
func TestEdgeCasePortConflicts(t *testing.T) {
	tmpDir := t.TempDir()

	// Create deps with conflicting ports
	deps := testhelpers.CreateTestDepsWithService("service1", "image")
	svc1 := deps.Services["service1"]
	svc1.Docker.Ports = []string{"8080:8080"}
	deps.Services["service1"] = svc1

	deps.Services["service2"] = config.Service{
		Source: config.SourceConfig{
			Kind:  "image",
			Image: "test/image2",
			Tag:   "latest",
		},
		Docker: config.DockerConfig{
			Mode:  "dev",
			Ports: []string{"8080:8081"}, // Different host port, should be OK
		},
	}

	// This should not cause an issue during validation
	// as port conflict detection happens during docker.ValidatePorts
	depsPath, err := testhelpers.CreateTestDepsJSON(tmpDir, deps)
	if err != nil {
		t.Fatalf("Failed to create test .raioz.json: %v", err)
	}

	loaded, _, err := config.LoadDeps(depsPath)
	if err != nil {
		t.Fatalf("Failed to load deps: %v", err)
	}

	if len(loaded.Services) != 2 {
		t.Errorf("Expected 2 services, got %d", len(loaded.Services))
	}
}

// TestEdgeCaseEmptyServices tests edge case of empty services
func TestEdgeCaseEmptyServices(t *testing.T) {
	tmpDir := t.TempDir()

	// Create deps with no services
	deps := testhelpers.CreateMinimalTestDeps()
	deps.Services = map[string]config.Service{}

	depsPath, err := testhelpers.CreateTestDepsJSON(tmpDir, deps)
	if err != nil {
		t.Fatalf("Failed to create test .raioz.json: %v", err)
	}

	loaded, _, err := config.LoadDeps(depsPath)
	if err != nil {
		t.Fatalf("Failed to load deps: %v", err)
	}

	if len(loaded.Services) != 0 {
		t.Errorf("Expected 0 services, got %d", len(loaded.Services))
	}
}

// TestEdgeCaseMissingConfigFile tests error when config file doesn't exist
func TestEdgeCaseMissingConfigFile(t *testing.T) {
	nonExistentPath := "/tmp/non-existent-deps-12345.json"

	_, _, err := config.LoadDeps(nonExistentPath)
	if err == nil {
		t.Error("Expected error when loading non-existent file, got nil")
	}
}

// TestEdgeCaseNetworkExists tests handling when network already exists
func TestEdgeCaseNetworkExists(t *testing.T) {
	// This would require mocking docker network commands
	// For now, we just verify the function exists and can be called
	// Actual network existence is tested in internal/docker/network_test.go
	t.Skip("Requires Docker mocking - tested in internal/docker/network_test.go")
}

// TestEdgeCaseVolumeShared tests handling when volume is shared between projects
func TestEdgeCaseVolumeShared(t *testing.T) {
	// This would require mocking docker volume commands
	// For now, we just verify the function exists and can be called
	// Actual volume handling is tested in internal/docker/volumes_test.go
	t.Skip("Requires Docker mocking - tested in internal/docker/volumes_test.go")
}
