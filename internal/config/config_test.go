package config

import (
	"os"
	"testing"
)

func TestLoadDeps(t *testing.T) {
	// Create a temporary .raioz.json file
	content := `{
		"schemaVersion": "1.0",
		"project": {
			"name": "test-project",
			"network": "test-network"
		},
		"services": {
			"test-service": {
				"source": {
					"kind": "git",
					"repo": "git@github.com:test/repo.git",
					"branch": "main",
					"path": "services/test"
				},
				"docker": {
					"mode": "dev",
					"ports": ["3000:3000"],
					"dockerfile": "Dockerfile"
				}
			}
		},
		"infra": {},
		"env": {
			"useGlobal": true,
			"files": []
		}
	}`

	tmpfile, err := os.CreateTemp("", "deps*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	deps, _, err := LoadDeps(tmpfile.Name())
	if err != nil {
		t.Fatalf("LoadDeps failed: %v", err)
	}

	if deps.Project.Name != "test-project" {
		t.Errorf("Expected project name 'test-project', got '%s'", deps.Project.Name)
	}

	if deps.Network.GetName() != "test-network" {
		t.Errorf("Expected network 'test-network', got '%s'", deps.Network.GetName())
	}

	if len(deps.Services) != 1 {
		t.Errorf("Expected 1 service, got %d", len(deps.Services))
	}

	svc, exists := deps.Services["test-service"]
	if !exists {
		t.Fatal("Service 'test-service' not found")
	}

	if svc.Source.Kind != "git" {
		t.Errorf("Expected source kind 'git', got '%s'", svc.Source.Kind)
	}
}

func TestFilterByProfile(t *testing.T) {
	deps := &Deps{
		Services: map[string]Service{
			"frontend-svc": {
				Profiles: []string{"frontend"},
			},
			"backend-svc": {
				Profiles: []string{"backend"},
			},
			"shared-svc": {
				Profiles: []string{},
			},
		},
		Infra: map[string]Infra{},
	}

	// Test frontend profile
	frontendDeps := FilterByProfile(deps, "frontend")
	if len(frontendDeps.Services) != 2 {
		t.Errorf("Expected 2 services for frontend profile, got %d", len(frontendDeps.Services))
	}
	if _, exists := frontendDeps.Services["frontend-svc"]; !exists {
		t.Error("frontend-svc should be included")
	}
	if _, exists := frontendDeps.Services["shared-svc"]; !exists {
		t.Error("shared-svc should be included")
	}
	if _, exists := frontendDeps.Services["backend-svc"]; exists {
		t.Error("backend-svc should not be included")
	}

	// Test backend profile
	backendDeps := FilterByProfile(deps, "backend")
	if len(backendDeps.Services) != 2 {
		t.Errorf("Expected 2 services for backend profile, got %d", len(backendDeps.Services))
	}
	if _, exists := backendDeps.Services["backend-svc"]; !exists {
		t.Error("backend-svc should be included")
	}
	if _, exists := backendDeps.Services["shared-svc"]; !exists {
		t.Error("shared-svc should be included")
	}
	if _, exists := backendDeps.Services["frontend-svc"]; exists {
		t.Error("frontend-svc should not be included")
	}
}
