package state

import (
	"os"
	"path/filepath"
	"testing"

	"raioz/internal/config"
	"raioz/internal/workspace"
)

func TestSaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	ws := &workspace.Workspace{
		Root:        tmpDir,
		ServicesDir: filepath.Join(tmpDir, "services"),
	}

	deps := &config.Deps{
		SchemaVersion: "1.0",
		Project: config.Project{
			Name:    "test-project",
			Network: "test-network",
		},
		Services: map[string]config.Service{},
		Infra:    map[string]config.Infra{},
		Env: config.EnvConfig{
			UseGlobal: true,
			Files:     []string{},
		},
	}

	// Save
	if err := Save(ws, deps); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Load
	loaded, err := Load(ws)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded == nil {
		t.Fatal("Loaded deps is nil")
	}

	if loaded.Project.Name != deps.Project.Name {
		t.Errorf("Expected project name %s, got %s", deps.Project.Name, loaded.Project.Name)
	}

	if loaded.Project.Network != deps.Project.Network {
		t.Errorf("Expected network %s, got %s", deps.Project.Network, loaded.Project.Network)
	}
}

func TestExists(t *testing.T) {
	tmpDir := t.TempDir()
	ws := &workspace.Workspace{
		Root:        tmpDir,
		ServicesDir: filepath.Join(tmpDir, "services"),
	}

	// Should not exist initially
	if Exists(ws) {
		t.Error("State should not exist initially")
	}

	// Create state
	deps := &config.Deps{
		SchemaVersion: "1.0",
		Project: config.Project{
			Name:    "test",
			Network: "test",
		},
		Services: map[string]config.Service{},
		Infra:    map[string]config.Infra{},
		Env:      config.EnvConfig{},
	}

	if err := Save(ws, deps); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Should exist now
	if !Exists(ws) {
		t.Error("State should exist after saving")
	}
}

func TestLoadNonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	ws := &workspace.Workspace{
		Root:        tmpDir,
		ServicesDir: filepath.Join(tmpDir, "services"),
	}

	// Load non-existent state
	loaded, err := Load(ws)
	if err != nil {
		t.Fatalf("Load should not return error for non-existent file: %v", err)
	}

	if loaded != nil {
		t.Error("Load should return nil for non-existent file")
	}
}

func TestSaveWithServices(t *testing.T) {
	tmpDir := t.TempDir()
	ws := &workspace.Workspace{
		Root:        tmpDir,
		ServicesDir: filepath.Join(tmpDir, "services"),
	}

	deps := &config.Deps{
		SchemaVersion: "1.0",
		Project: config.Project{
			Name:    "test-project",
			Network: "test-network",
		},
		Services: map[string]config.Service{
			"api": {
				Source: config.SourceConfig{
					Kind:   "git",
					Repo:   "git@github.com:org/api.git",
					Branch: "main",
					Path:   "./services/api",
				},
				Docker: config.DockerConfig{
					Mode:  "dev",
					Ports: []string{"3000:3000"},
				},
			},
		},
		Infra: map[string]config.Infra{
			"database": {
				Image: "postgres",
				Tag:   "15",
			},
		},
		Env: config.EnvConfig{
			UseGlobal: true,
			Files:     []string{"global"},
		},
	}

	// Save
	if err := Save(ws, deps); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Load and verify
	loaded, err := Load(ws)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if len(loaded.Services) != 1 {
		t.Errorf("Expected 1 service, got %d", len(loaded.Services))
	}
	if len(loaded.Infra) != 1 {
		t.Errorf("Expected 1 infra, got %d", len(loaded.Infra))
	}
	if loaded.Services["api"].Source.Repo != "git@github.com:org/api.git" {
		t.Errorf("Expected repo git@github.com:org/api.git, got %s", loaded.Services["api"].Source.Repo)
	}
}

func TestSavePermissions(t *testing.T) {
	tmpDir := t.TempDir()
	ws := &workspace.Workspace{
		Root:        tmpDir,
		ServicesDir: filepath.Join(tmpDir, "services"),
	}

	deps := &config.Deps{
		SchemaVersion: "1.0",
		Project: config.Project{
			Name:    "test",
			Network: "test",
		},
		Services: map[string]config.Service{},
		Infra:    map[string]config.Infra{},
		Env:      config.EnvConfig{},
	}

	// Save
	if err := Save(ws, deps); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify file permissions (should be 0600)
	statePath := filepath.Join(ws.Root, stateFileName)
	info, err := os.Stat(statePath)
	if err != nil {
		t.Fatalf("Failed to stat state file: %v", err)
	}

	mode := info.Mode().Perm()
	expectedMode := os.FileMode(0600)
	if mode != expectedMode {
		t.Errorf("Expected file mode %o, got %o", expectedMode, mode)
	}
}
