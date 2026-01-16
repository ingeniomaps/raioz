package root

import (
	"os"
	"path/filepath"
	"testing"

	"raioz/internal/config"
	"raioz/internal/state"
	testhelpers "raioz/internal/testing"
	"raioz/internal/workspace"
)

func TestDetectAssistedServiceDrift(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "raioz-drift-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create workspace
	ws := &workspace.Workspace{
		Root:                tmpDir,
		LocalServicesDir:    filepath.Join(tmpDir, "local"),
		ReadonlyServicesDir: filepath.Join(tmpDir, "readonly"),
		ServicesDir:         filepath.Join(tmpDir, "services"),
		EnvDir:              filepath.Join(tmpDir, "env"),
	}

	// Create service directory
	serviceName := "test-service"
	servicePath := filepath.Join(ws.LocalServicesDir, "services", serviceName)
	if err := os.MkdirAll(servicePath, 0755); err != nil {
		t.Fatalf("Failed to create service dir: %v", err)
	}

	// Create root config with assisted service
	rootConfig := &RootConfig{
		SchemaVersion: "1.0",
		Project: config.Project{
			Name:    "test-project",
			Network: "test-network",
		},
		Services: map[string]config.Service{
			serviceName: {
				Source: config.SourceConfig{
					Kind:   "git",
					Repo:   "git@github.com:test/repo.git",
					Branch: "main",
					Path:   filepath.Join("services", serviceName),
				},
				Docker: config.DockerConfig{
					Mode:  "dev",
					Ports: []string{"3000:3000"},
				},
			},
		},
		Metadata: map[string]ServiceMetadata{
			serviceName: {
				Origin:  OriginAssisted,
				AddedBy: "parent-service",
				Reason:  "dependency assist",
			},
		},
	}

	// Test case 1: No drift (service .raioz.json matches root config)
	t.Run("no_drift", func(t *testing.T) {
		// Create service .raioz.json matching root config
		serviceDeps := &config.Deps{
			SchemaVersion: "1.0",
			Project: config.Project{
				Name:    "test-project",
				Network: "test-network",
			},
			Services: map[string]config.Service{
				serviceName: {
					Source: config.SourceConfig{
						Kind:   "git",
						Repo:   "git@github.com:test/repo.git",
						Branch: "main",
						Path:   filepath.Join("services", serviceName),
					},
					Docker: config.DockerConfig{
						Mode:  "dev",
						Ports: []string{"3000:3000"},
					},
				},
			},
			Infra: map[string]config.Infra{},
			Env: config.EnvConfig{
				UseGlobal: true,
				Files:     []string{},
			},
		}

		serviceConfigPath, err := testhelpers.CreateTestDepsJSON(servicePath, serviceDeps)
		if err != nil {
			t.Fatalf("Failed to create service config: %v", err)
		}
		// Rename to .raioz.json
		raiozPath := filepath.Join(servicePath, ".raioz.json")
		if err := os.Rename(serviceConfigPath, raiozPath); err != nil {
			t.Fatalf("Failed to rename config: %v", err)
		}

		drifts, err := DetectAssistedServiceDrift(rootConfig, ws)
		if err != nil {
			t.Fatalf("DetectAssistedServiceDrift() error = %v", err)
		}

		if len(drifts) != 0 {
			t.Errorf("DetectAssistedServiceDrift() found %d drifts, expected 0", len(drifts))
		}
	})

	// Test case 2: Drift detected (branch changed)
	t.Run("drift_branch_changed", func(t *testing.T) {
		// Create service .raioz.json with different branch
		serviceDeps := &config.Deps{
			SchemaVersion: "1.0",
			Project: config.Project{
				Name:    "test-project",
				Network: "test-network",
			},
			Services: map[string]config.Service{
				serviceName: {
					Source: config.SourceConfig{
						Kind:   "git",
						Repo:   "git@github.com:test/repo.git",
						Branch: "develop", // Different branch
						Path:   filepath.Join("services", serviceName),
					},
					Docker: config.DockerConfig{
						Mode:  "dev",
						Ports: []string{"3000:3000"},
					},
				},
			},
			Infra: map[string]config.Infra{},
			Env: config.EnvConfig{
				UseGlobal: true,
				Files:     []string{},
			},
		}

		serviceConfigPath, err := testhelpers.CreateTestDepsJSON(servicePath, serviceDeps)
		if err != nil {
			t.Fatalf("Failed to create service config: %v", err)
		}
		// Rename to .raioz.json
		raiozPath := filepath.Join(servicePath, ".raioz.json")
		if err := os.Rename(serviceConfigPath, raiozPath); err != nil {
			t.Fatalf("Failed to rename config: %v", err)
		}

		drifts, err := DetectAssistedServiceDrift(rootConfig, ws)
		if err != nil {
			t.Fatalf("DetectAssistedServiceDrift() error = %v", err)
		}

		if len(drifts) != 1 {
			t.Fatalf("DetectAssistedServiceDrift() found %d drifts, expected 1", len(drifts))
		}

		drift := drifts[0]
		if drift.ServiceName != serviceName {
			t.Errorf("Drift.ServiceName = %s, want %s", drift.ServiceName, serviceName)
		}

		if len(drift.Differences) == 0 {
			t.Error("Drift.Differences is empty, expected at least one difference")
		}

		// Check that branch difference is detected
		foundBranchDiff := false
		for _, change := range drift.Differences {
			if change.Field == "source.branch" {
				foundBranchDiff = true
				if change.OldValue != "main" {
					t.Errorf("Change.OldValue = %s, want main", change.OldValue)
				}
				if change.NewValue != "develop" {
					t.Errorf("Change.NewValue = %s, want develop", change.NewValue)
				}
			}
		}

		if !foundBranchDiff {
			t.Error("Branch difference not detected")
		}
	})

	// Test case 3: Drift detected (ports changed)
	t.Run("drift_ports_changed", func(t *testing.T) {
		// Create service .raioz.json with different ports
		serviceDeps := &config.Deps{
			SchemaVersion: "1.0",
			Project: config.Project{
				Name:    "test-project",
				Network: "test-network",
			},
			Services: map[string]config.Service{
				serviceName: {
					Source: config.SourceConfig{
						Kind:   "git",
						Repo:   "git@github.com:test/repo.git",
						Branch: "main",
						Path:   filepath.Join("services", serviceName),
					},
					Docker: config.DockerConfig{
						Mode:  "dev",
						Ports: []string{"8080:8080"}, // Different ports
					},
				},
			},
			Infra: map[string]config.Infra{},
			Env: config.EnvConfig{
				UseGlobal: true,
				Files:     []string{},
			},
		}

		serviceConfigPath, err := testhelpers.CreateTestDepsJSON(servicePath, serviceDeps)
		if err != nil {
			t.Fatalf("Failed to create service config: %v", err)
		}
		// Rename to .raioz.json
		raiozPath := filepath.Join(servicePath, ".raioz.json")
		if err := os.Rename(serviceConfigPath, raiozPath); err != nil {
			t.Fatalf("Failed to rename config: %v", err)
		}

		drifts, err := DetectAssistedServiceDrift(rootConfig, ws)
		if err != nil {
			t.Fatalf("DetectAssistedServiceDrift() error = %v", err)
		}

		if len(drifts) != 1 {
			t.Fatalf("DetectAssistedServiceDrift() found %d drifts, expected 1", len(drifts))
		}

		// Check that ports difference is detected
		foundPortsDiff := false
		for _, change := range drifts[0].Differences {
			if change.Field == "docker.ports" {
				foundPortsDiff = true
				break
			}
		}

		if !foundPortsDiff {
			t.Error("Ports difference not detected")
		}
	})

	// Test case 4: Service not added via assist (should be skipped)
	t.Run("skip_non_assisted", func(t *testing.T) {
		rootConfigWithRootService := &RootConfig{
			SchemaVersion: "1.0",
			Project: config.Project{
				Name:    "test-project",
				Network: "test-network",
			},
			Services: map[string]config.Service{
				serviceName: {
					Source: config.SourceConfig{
						Kind:   "git",
						Repo:   "git@github.com:test/repo.git",
						Branch: "main",
						Path:   filepath.Join("services", serviceName),
					},
					Docker: config.DockerConfig{
						Mode:  "dev",
						Ports: []string{"3000:3000"},
					},
				},
			},
			Metadata: map[string]ServiceMetadata{
				serviceName: {
					Origin:  OriginRoot, // Not assisted
					AddedBy: ".raioz.json",
				},
			},
		}

		drifts, err := DetectAssistedServiceDrift(rootConfigWithRootService, ws)
		if err != nil {
			t.Fatalf("DetectAssistedServiceDrift() error = %v", err)
		}

		if len(drifts) != 0 {
			t.Errorf("DetectAssistedServiceDrift() found %d drifts, expected 0 (service not assisted)", len(drifts))
		}
	})

	// Test case 5: Service .raioz.json doesn't exist (should be skipped)
	t.Run("skip_no_service_config", func(t *testing.T) {
		// Remove service .raioz.json
		raiozPath := filepath.Join(servicePath, ".raioz.json")
		os.Remove(raiozPath)

		drifts, err := DetectAssistedServiceDrift(rootConfig, ws)
		if err != nil {
			t.Fatalf("DetectAssistedServiceDrift() error = %v", err)
		}

		if len(drifts) != 0 {
			t.Errorf("DetectAssistedServiceDrift() found %d drifts, expected 0 (no service config)", len(drifts))
		}
	})
}

func TestFormatDrift(t *testing.T) {
	drift := ServiceDrift{
		ServiceName: "test-service",
		ServicePath: "/path/to/service/.raioz.json",
		Differences: []state.ConfigChange{
			{
				Type:     "service",
				Name:     "test-service",
				Field:    "source.branch",
				OldValue: "main",
				NewValue: "develop",
			},
			{
				Type:     "service",
				Name:     "test-service",
				Field:    "docker.ports",
				OldValue: "[3000:3000]",
				NewValue: "[8080:8080]",
			},
		},
	}

	formatted := FormatDrift(drift)
	if formatted == "" {
		t.Error("FormatDrift() returned empty string")
	}

	// Check that service name is included
	if !contains(formatted, "test-service") {
		t.Error("FormatDrift() output doesn't contain service name")
	}

	// Check that differences are included
	if !contains(formatted, "source.branch") {
		t.Error("FormatDrift() output doesn't contain branch difference")
	}
	if !contains(formatted, "docker.ports") {
		t.Error("FormatDrift() output doesn't contain ports difference")
	}
}

func TestFormatDrifts(t *testing.T) {
	drifts := []ServiceDrift{
		{
			ServiceName: "service1",
			ServicePath: "/path/to/service1/.raioz.json",
			Differences: []state.ConfigChange{
				{
					Type:     "service",
					Name:     "service1",
					Field:    "source.branch",
					OldValue: "main",
					NewValue: "develop",
				},
			},
		},
		{
			ServiceName: "service2",
			ServicePath: "/path/to/service2/.raioz.json",
			Differences: []state.ConfigChange{
				{
					Type:     "service",
					Name:     "service2",
					Field:    "docker.ports",
					OldValue: "[3000:3000]",
					NewValue: "[8080:8080]",
				},
			},
		},
	}

	formatted := FormatDrifts(drifts)
	if formatted == "" {
		t.Error("FormatDrifts() returned empty string")
	}

	// Check that both services are included
	if !contains(formatted, "service1") {
		t.Error("FormatDrifts() output doesn't contain service1")
	}
	if !contains(formatted, "service2") {
		t.Error("FormatDrifts() output doesn't contain service2")
	}

	// Test with empty drifts
	emptyFormatted := FormatDrifts([]ServiceDrift{})
	if emptyFormatted != "" {
		t.Errorf("FormatDrifts([]) returned %q, expected empty string", emptyFormatted)
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > len(substr) && (s[:len(substr)] == substr ||
		s[len(s)-len(substr):] == substr ||
		containsMiddle(s, substr))))
}

func containsMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
