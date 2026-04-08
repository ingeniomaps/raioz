package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"raioz/internal/config"
	testhelpers "raioz/internal/testing"
)

func TestLoadDeps_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("completely malformed JSON", func(t *testing.T) {
		path := filepath.Join(tmpDir, "malformed.json")
		os.WriteFile(path, []byte("{ invalid json }"), 0644)

		_, _, err := config.LoadDeps(path)
		if err == nil {
			t.Error("Expected error for malformed JSON, got nil")
		}
	})

	t.Run("JSON with syntax error", func(t *testing.T) {
		path := filepath.Join(tmpDir, "syntax-error.json")
		os.WriteFile(path, []byte(`{"schemaVersion": "1.0", "project": {`), 0644)

		_, _, err := config.LoadDeps(path)
		if err == nil {
			t.Error("Expected error for JSON syntax error, got nil")
		}
	})

	t.Run("empty file", func(t *testing.T) {
		path := filepath.Join(tmpDir, "empty.json")
		os.WriteFile(path, []byte(""), 0644)

		_, _, err := config.LoadDeps(path)
		if err == nil {
			t.Error("Expected error for empty file, got nil")
		}
	})

	t.Run("non-existent file", func(t *testing.T) {
		_, _, err := config.LoadDeps("/tmp/non-existent-deps-12345.json")
		if err == nil {
			t.Error("Expected error when loading non-existent file, got nil")
		}
	})
}

func TestLoadDeps_MissingRequiredFields(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("missing project name", func(t *testing.T) {
		deps := testhelpers.CreateMinimalTestDeps()
		deps.Project.Name = ""
		depsPath, _ := testhelpers.CreateTestDepsJSON(tmpDir, deps)

		// Loading may succeed; validation catches missing name
		_, _, _ = config.LoadDeps(depsPath)
	})

	t.Run("missing network", func(t *testing.T) {
		deps := testhelpers.CreateMinimalTestDeps()
		deps.Network = config.NetworkConfig{Name: "", IsObject: false}
		depsPath, _ := testhelpers.CreateTestDepsJSON(tmpDir, deps)

		_, _, _ = config.LoadDeps(depsPath)
	})
}

func TestLoadDeps_InvalidServiceConfig(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("invalid source kind", func(t *testing.T) {
		deps := testhelpers.CreateMinimalTestDeps()
		deps.Services["invalid-service"] = config.Service{
			Source: config.SourceConfig{Kind: "invalid-kind"},
		}
		depsPath, _ := testhelpers.CreateTestDepsJSON(tmpDir, deps)

		loaded, _, err := config.LoadDeps(depsPath)
		if err != nil {
			t.Fatalf("Load should succeed even with invalid config: %v", err)
		}
		_ = loaded
	})

	t.Run("empty source", func(t *testing.T) {
		deps := testhelpers.CreateMinimalTestDeps()
		deps.Services["empty-source"] = config.Service{
			Source: config.SourceConfig{},
		}
		depsPath, _ := testhelpers.CreateTestDepsJSON(tmpDir, deps)

		loaded, _, err := config.LoadDeps(depsPath)
		if err != nil {
			t.Fatalf("Load should succeed: %v", err)
		}
		_ = loaded
	})
}

func TestLoadDeps_EmptyValues(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("empty services map", func(t *testing.T) {
		deps := testhelpers.CreateMinimalTestDeps()
		deps.Services = map[string]config.Service{}
		depsPath, _ := testhelpers.CreateTestDepsJSON(tmpDir, deps)

		loaded, _, err := config.LoadDeps(depsPath)
		if err != nil {
			t.Fatalf("Failed to load deps: %v", err)
		}
		if len(loaded.Services) != 0 {
			t.Errorf("Expected 0 services, got %d", len(loaded.Services))
		}
	})

	t.Run("empty infra map", func(t *testing.T) {
		deps := testhelpers.CreateMinimalTestDeps()
		deps.Infra = map[string]config.InfraEntry{}
		depsPath, _ := testhelpers.CreateTestDepsJSON(tmpDir, deps)

		loaded, _, err := config.LoadDeps(depsPath)
		if err != nil {
			t.Fatalf("Failed to load deps: %v", err)
		}
		if len(loaded.Infra) != 0 {
			t.Errorf("Expected 0 infra, got %d", len(loaded.Infra))
		}
	})

	t.Run("empty env files", func(t *testing.T) {
		deps := testhelpers.CreateMinimalTestDeps()
		deps.Env.Files = []string{}
		depsPath, _ := testhelpers.CreateTestDepsJSON(tmpDir, deps)

		loaded, _, err := config.LoadDeps(depsPath)
		if err != nil {
			t.Fatalf("Failed to load deps: %v", err)
		}
		if len(loaded.Env.Files) != 0 {
			t.Errorf("Expected 0 env files, got %d", len(loaded.Env.Files))
		}
	})
}

func TestLoadDeps_SpecialCharacters(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("service name with special characters", func(t *testing.T) {
		deps := testhelpers.CreateMinimalTestDeps()
		deps.Services["service_123-test"] = config.Service{
			Source: config.SourceConfig{
				Kind:  "image",
				Image: "nginx",
				Tag:   "alpine",
			},
		}
		depsPath, _ := testhelpers.CreateTestDepsJSON(tmpDir, deps)

		loaded, _, err := config.LoadDeps(depsPath)
		if err != nil {
			t.Fatalf("Load failed: %v", err)
		}
		if _, exists := loaded.Services["service_123-test"]; !exists {
			t.Error("Service with special characters should exist")
		}
	})
}

func TestLoadDeps_VeryLongServiceName(t *testing.T) {
	tmpDir := t.TempDir()

	longServiceName := "service-" + strings.Repeat("x", 100)
	deps := testhelpers.CreateMinimalTestDeps()
	deps.Services[longServiceName] = config.Service{
		Source: config.SourceConfig{
			Kind:  "image",
			Image: "nginx",
			Tag:   "alpine",
		},
	}
	depsPath, _ := testhelpers.CreateTestDepsJSON(tmpDir, deps)

	loaded, _, err := config.LoadDeps(depsPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if _, exists := loaded.Services[longServiceName]; !exists {
		t.Error("Service with long name should exist")
	}
}

func TestLoadDeps_UnicodeCharacters(t *testing.T) {
	tmpDir := t.TempDir()

	deps := testhelpers.CreateMinimalTestDeps()
	deps.Project.Name = "test-项目-123"
	deps.Network = config.NetworkConfig{Name: "test-network-网络", IsObject: false}
	depsPath, _ := testhelpers.CreateTestDepsJSON(tmpDir, deps)

	loaded, _, err := config.LoadDeps(depsPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if loaded.Project.Name != "test-项目-123" {
		t.Errorf("Expected unicode project name, got %s", loaded.Project.Name)
	}
}

func TestLoadDeps_PortConflicts(t *testing.T) {
	tmpDir := t.TempDir()

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
		Docker: &config.DockerConfig{
			Mode:  "dev",
			Ports: []string{"8080:8081"},
		},
	}

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

func TestLoadDeps_WithInvalidSchema(t *testing.T) {
	tmpDir := t.TempDir()

	malformedPath, err := testhelpers.CreateMalformedDepsJSON(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create malformed .raioz.json: %v", err)
	}

	_, _, err = config.LoadDeps(malformedPath)
	if err == nil {
		t.Error("Expected error when loading malformed JSON, got nil")
	}

	invalidPath, err := testhelpers.CreateInvalidDepsJSON(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create invalid .raioz.json: %v", err)
	}

	// Loading should succeed; validation catches schema errors
	_, _, _ = config.LoadDeps(invalidPath)
}
