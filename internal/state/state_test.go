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
		Network:       config.NetworkConfig{Name: "test-network"},
		Project:       config.Project{Name: "test-project"},
		Services:      map[string]config.Service{},
		Infra:         map[string]config.InfraEntry{},
		Env:           config.EnvConfig{UseGlobal: true, Files: []string{}},
	}

	if err := Save(ws, deps); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

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

	if loaded.Network.GetName() != deps.Network.GetName() {
		t.Errorf("Expected network %s, got %s", deps.Network.GetName(), loaded.Network.GetName())
	}
}

func TestExists(t *testing.T) {
	tmpDir := t.TempDir()
	ws := &workspace.Workspace{
		Root:        tmpDir,
		ServicesDir: filepath.Join(tmpDir, "services"),
	}

	if Exists(ws) {
		t.Error("State should not exist initially")
	}

	deps := &config.Deps{
		SchemaVersion: "1.0",
		Network:       config.NetworkConfig{Name: "test"},
		Project:       config.Project{Name: "test"},
		Services:      map[string]config.Service{},
		Infra:         map[string]config.InfraEntry{},
		Env:           config.EnvConfig{},
	}

	if err := Save(ws, deps); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

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
		Network:       config.NetworkConfig{Name: "test-network"},
		Project:       config.Project{Name: "test-project"},
		Services: map[string]config.Service{
			"api": {
				Source: config.SourceConfig{Kind: "git", Repo: "git@github.com:org/api.git", Branch: "main", Path: "./services/api"},
				Docker: &config.DockerConfig{Mode: "dev", Ports: []string{"3000:3000"}},
			},
		},
		Infra: map[string]config.InfraEntry{
			"database": {Inline: &config.Infra{Image: "postgres", Tag: "15"}},
		},
		Env: config.EnvConfig{UseGlobal: true, Files: []string{"global"}},
	}

	if err := Save(ws, deps); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

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
		Network:       config.NetworkConfig{Name: "test"},
		Project:       config.Project{Name: "test"},
		Services:      map[string]config.Service{},
		Infra:         map[string]config.InfraEntry{},
		Env:           config.EnvConfig{},
	}

	if err := Save(ws, deps); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

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
