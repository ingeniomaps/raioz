package testing

import (
	"testing"

	"raioz/internal/config"
)

func TestCreateMinimalTestDeps(t *testing.T) {
	deps := CreateMinimalTestDeps()

	if deps.SchemaVersion != "1.0" {
		t.Errorf("Expected schema version 1.0, got %s", deps.SchemaVersion)
	}

	if deps.Project.Name != "test-project" {
		t.Errorf("Expected project name 'test-project', got %s", deps.Project.Name)
	}

	if len(deps.Services) != 0 {
		t.Errorf("Expected 0 services, got %d", len(deps.Services))
	}

	if len(deps.Infra) != 0 {
		t.Errorf("Expected 0 infra, got %d", len(deps.Infra))
	}
}

func TestCreateTestDepsWithService(t *testing.T) {
	tests := []struct {
		name       string
		serviceName string
		sourceKind string
	}{
		{"git service", "test-service", "git"},
		{"image service", "test-service", "image"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deps := CreateTestDepsWithService(tt.serviceName, tt.sourceKind)

			svc, exists := deps.Services[tt.serviceName]
			if !exists {
				t.Fatalf("Expected service %s to exist", tt.serviceName)
			}

			if svc.Source.Kind != tt.sourceKind {
				t.Errorf("Expected source kind %s, got %s",
					tt.sourceKind, svc.Source.Kind)
			}

			if svc.Docker.Mode != "dev" {
				t.Errorf("Expected docker mode 'dev', got %s", svc.Docker.Mode)
			}

			if tt.sourceKind == "git" {
				if svc.Source.Repo == "" {
					t.Error("Expected repo to be set for git service")
				}
				if svc.Source.Branch == "" {
					t.Error("Expected branch to be set for git service")
				}
				if svc.Source.Path == "" {
					t.Error("Expected path to be set for git service")
				}
			} else if tt.sourceKind == "image" {
				if svc.Source.Image == "" {
					t.Error("Expected image to be set for image service")
				}
				if svc.Source.Tag == "" {
					t.Error("Expected tag to be set for image service")
				}
			}
		})
	}
}

func TestCreateTestDepsWithInfra(t *testing.T) {
	deps := CreateTestDepsWithInfra("postgres")

	infra, exists := deps.Infra["postgres"]
	if !exists {
		t.Fatal("Expected infra 'postgres' to exist")
	}

	if infra.Inline.Image != "postgres" {
		t.Errorf("Expected image 'postgres', got %s", infra.Inline.Image)
	}

	if infra.Inline.Tag != "15" {
		t.Errorf("Expected tag '15', got %s", infra.Inline.Tag)
	}
}

func TestCreateTestDepsJSON(t *testing.T) {
	tmpDir := t.TempDir()
	deps := CreateMinimalTestDeps()

	depsPath, err := CreateTestDepsJSON(tmpDir, deps)
	if err != nil {
		t.Fatalf("Failed to create test .raioz.json: %v", err)
	}

	if depsPath == "" {
		t.Error("Expected deps path to be non-empty")
	}

	// Try to load it back
	loaded, _, err := config.LoadDeps(depsPath)
	if err != nil {
		t.Fatalf("Failed to load created .raioz.json: %v", err)
	}

	if loaded.Project.Name != deps.Project.Name {
		t.Errorf("Expected project name %s, got %s",
			deps.Project.Name, loaded.Project.Name)
	}
}
