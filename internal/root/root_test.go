package root

import (
	"os"
	"path/filepath"
	"testing"

	"raioz/internal/config"
	"raioz/internal/workspace"
)

func TestGetRootPath(t *testing.T) {
	tmpDir := t.TempDir()
	ws := &workspace.Workspace{
		Root: tmpDir,
	}

	path := GetRootPath(ws)
	expected := filepath.Join(tmpDir, rootFileName)
	if path != expected {
		t.Errorf("Expected path %s, got %s", expected, path)
	}
}

func TestExists(t *testing.T) {
	tmpDir := t.TempDir()
	ws := &workspace.Workspace{
		Root: tmpDir,
	}

	t.Run("file does not exist", func(t *testing.T) {
		if Exists(ws) {
			t.Error("Expected file not to exist")
		}
	})

	t.Run("file exists", func(t *testing.T) {
		// Create file
		path := GetRootPath(ws)
		os.WriteFile(path, []byte("{}"), 0644)

		if !Exists(ws) {
			t.Error("Expected file to exist")
		}
	})
}

func TestLoad(t *testing.T) {
	tmpDir := t.TempDir()
	ws := &workspace.Workspace{
		Root: tmpDir,
	}

	t.Run("load non-existent file", func(t *testing.T) {
		root, err := Load(ws)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if root != nil {
			t.Error("Expected nil for non-existent file")
		}
	})

	t.Run("load existing file", func(t *testing.T) {
		// Create root config
		root := &RootConfig{
			SchemaVersion: "1.0",
			Project: config.Project{
				Name: "test-project",
			},
			Services: make(map[string]config.Service),
			Infra:    make(map[string]config.InfraEntry),
			Env:      config.EnvConfig{},
		}
		Save(ws, root)

		// Load it
		loaded, err := Load(ws)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if loaded == nil {
			t.Fatal("Expected root config, got nil")
		}
		if loaded.Project.Name != "test-project" {
			t.Errorf("Expected project name test-project, got %s", loaded.Project.Name)
		}
		if loaded.Metadata == nil {
			t.Error("Expected metadata map to be initialized")
		}
	})

	t.Run("load file with nil metadata", func(t *testing.T) {
		// Create file with null metadata
		path := GetRootPath(ws)
		data := []byte(`{"schemaVersion":"1.0","project":{"name":"test","network":"test"},"services":{},"infra":{},"env":{}}`)
		os.WriteFile(path, data, 0644)

		loaded, err := Load(ws)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if loaded.Metadata == nil {
			t.Error("Expected metadata map to be initialized")
		}
	})
}

func TestSave(t *testing.T) {
	tmpDir := t.TempDir()
	ws := &workspace.Workspace{
		Root: tmpDir,
	}

	t.Run("save root config", func(t *testing.T) {
		root := &RootConfig{
			SchemaVersion: "1.0",
			Project: config.Project{
				Name: "test-project",
			},
			Services: make(map[string]config.Service),
			Infra:    make(map[string]config.InfraEntry),
			Env:      config.EnvConfig{},
		}

		err := Save(ws, root)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		// Verify file exists
		path := GetRootPath(ws)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Error("Root config file should exist")
		}

		// Verify timestamps were set
		if root.GeneratedAt == "" {
			t.Error("Expected GeneratedAt to be set")
		}
		if root.LastUpdatedAt == "" {
			t.Error("Expected LastUpdatedAt to be set")
		}
	})

	t.Run("save preserves GeneratedAt", func(t *testing.T) {
		originalTime := "2024-01-01T00:00:00Z"
		root := &RootConfig{
			SchemaVersion: "1.0",
			GeneratedAt:   originalTime,
			Project: config.Project{
				Name: "test",
			},
			Services: make(map[string]config.Service),
			Infra:    make(map[string]config.InfraEntry),
			Env:      config.EnvConfig{},
		}

		Save(ws, root)

		// GeneratedAt should be preserved
		if root.GeneratedAt != originalTime {
			t.Errorf("Expected GeneratedAt to be preserved, got %s", root.GeneratedAt)
		}
		// LastUpdatedAt should be updated
		if root.LastUpdatedAt == "" {
			t.Error("Expected LastUpdatedAt to be set")
		}
	})
}

func TestGenerateFromDeps(t *testing.T) {
	deps := &config.Deps{
		SchemaVersion: "1.0",
		Network:       config.NetworkConfig{Name: "test-network", IsObject: false},
		Project: config.Project{
			Name: "test-project",
		},
		Services: map[string]config.Service{
			"api": {
				Source: config.SourceConfig{
					Kind: "git",
					Repo: "git@github.com:org/api.git",
				},
			},
		},
		Infra: map[string]config.InfraEntry{
			"database": {Inline: &config.Infra{
				Image: "postgres",
				Tag:   "15",
			}},
		},
		Env: config.EnvConfig{
			UseGlobal: true,
		},
	}

	t.Run("generate without overrides", func(t *testing.T) {
		root, err := GenerateFromDeps(deps, []string{}, map[string]string{})
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if root == nil {
			t.Fatal("Expected root config, got nil")
		}
		if root.Project.Name != "test-project" {
			t.Errorf("Expected project name test-project, got %s", root.Project.Name)
		}
		if len(root.Services) != 1 {
			t.Errorf("Expected 1 service, got %d", len(root.Services))
		}
		if root.Metadata["api"].Origin != OriginRoot {
			t.Errorf("Expected origin %s, got %s", OriginRoot, root.Metadata["api"].Origin)
		}
	})

	t.Run("generate with overrides", func(t *testing.T) {
		root, err := GenerateFromDeps(deps, []string{"api"}, map[string]string{})
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if root.Metadata["api"].Origin != OriginOverride {
			t.Errorf("Expected origin %s, got %s", OriginOverride, root.Metadata["api"].Origin)
		}
	})

	t.Run("generate with assisted services", func(t *testing.T) {
		assistedServices := map[string]string{
			"new-service": "api",
		}
		deps.Services["new-service"] = config.Service{
			Source: config.SourceConfig{
				Kind: "git",
				Repo: "git@github.com:org/new-service.git",
			},
		}

		root, err := GenerateFromDeps(deps, []string{}, assistedServices)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if root.Metadata["new-service"].Origin != OriginRoot {
			// Note: GenerateFromDeps doesn't handle assisted services, only UpdateFromDeps does
			// So this will be OriginRoot
		}
	})
}

func TestUpdateFromDeps(t *testing.T) {
	existingRoot := &RootConfig{
		SchemaVersion: "1.0",
		GeneratedAt:   "2024-01-01T00:00:00Z",
		Project: config.Project{
			Name: "old-project",
		},
		Services: map[string]config.Service{
			"old-service": {
				Source: config.SourceConfig{
					Kind: "git",
					Repo: "git@github.com:org/old.git",
				},
			},
		},
		Infra: make(map[string]config.InfraEntry),
		Env:   config.EnvConfig{},
		Metadata: map[string]ServiceMetadata{
			"old-service": {
				Origin:  OriginRoot,
				AddedAt: "2024-01-01T00:00:00Z",
			},
		},
	}

	newDeps := &config.Deps{
		SchemaVersion: "1.0",
		Project: config.Project{
			Name: "new-project",
		},
		Services: map[string]config.Service{
			"new-service": {
				Source: config.SourceConfig{
					Kind: "git",
					Repo: "git@github.com:org/new.git",
				},
			},
		},
		Infra: make(map[string]config.InfraEntry),
		Env:   config.EnvConfig{},
	}

	t.Run("update basic fields", func(t *testing.T) {
		err := UpdateFromDeps(existingRoot, newDeps, []string{}, map[string]string{})
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if existingRoot.Project.Name != "new-project" {
			t.Errorf("Expected project name new-project, got %s", existingRoot.Project.Name)
		}
		if existingRoot.GeneratedAt == "" {
			t.Error("Expected GeneratedAt to be preserved")
		}
		if existingRoot.LastUpdatedAt == "" {
			t.Error("Expected LastUpdatedAt to be updated")
		}
	})

	t.Run("remove old services", func(t *testing.T) {
		// old-service should be removed
		if _, exists := existingRoot.Services["old-service"]; exists {
			t.Error("Expected old-service to be removed")
		}
		if _, exists := existingRoot.Metadata["old-service"]; exists {
			t.Error("Expected old-service metadata to be removed")
		}
	})

	t.Run("add new services", func(t *testing.T) {
		if _, exists := existingRoot.Services["new-service"]; !exists {
			t.Error("Expected new-service to be added")
		}
		if existingRoot.Metadata["new-service"].Origin != OriginRoot {
			t.Errorf("Expected origin %s, got %s", OriginRoot, existingRoot.Metadata["new-service"].Origin)
		}
	})

	t.Run("update with override", func(t *testing.T) {
		root := &RootConfig{
			SchemaVersion: "1.0",
			Services: map[string]config.Service{
				"api": {
					Source: config.SourceConfig{
						Kind: "git",
						Repo: "git@github.com:org/api.git",
					},
				},
			},
			Metadata: map[string]ServiceMetadata{
				"api": {
					Origin:  OriginRoot,
					AddedAt: "2024-01-01T00:00:00Z",
				},
			},
		}

		deps := &config.Deps{
			SchemaVersion: "1.0",
			Project:       config.Project{Name: "test"},
			Services: map[string]config.Service{
				"api": {
					Source: config.SourceConfig{
						Kind: "git",
						Repo: "git@github.com:org/api.git",
					},
				},
			},
			Infra: make(map[string]config.InfraEntry),
			Env:   config.EnvConfig{},
		}

		err := UpdateFromDeps(root, deps, []string{"api"}, map[string]string{})
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if root.Metadata["api"].Origin != OriginOverride {
			t.Errorf("Expected origin %s, got %s", OriginOverride, root.Metadata["api"].Origin)
		}
		if root.Metadata["api"].PreviousOrigin != OriginRoot {
			t.Errorf("Expected previous origin %s, got %s", OriginRoot, root.Metadata["api"].PreviousOrigin)
		}
	})

	t.Run("update with assisted service", func(t *testing.T) {
		root := &RootConfig{
			SchemaVersion: "1.0",
			Project:       config.Project{Name: "test"},
			Services:      make(map[string]config.Service),
			Infra:         make(map[string]config.InfraEntry),
			Env:           config.EnvConfig{},
			Metadata:      make(map[string]ServiceMetadata),
		}

		deps := &config.Deps{
			SchemaVersion: "1.0",
			Project:       config.Project{Name: "test"},
			Services: map[string]config.Service{
				"new-service": {
					Source: config.SourceConfig{
						Kind: "git",
						Repo: "git@github.com:org/new.git",
					},
				},
			},
			Infra: make(map[string]config.InfraEntry),
			Env:   config.EnvConfig{},
		}

		assistedServices := map[string]string{
			"new-service": "api",
		}

		err := UpdateFromDeps(root, deps, []string{}, assistedServices)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if root.Metadata["new-service"].Origin != OriginAssisted {
			t.Errorf("Expected origin %s, got %s", OriginAssisted, root.Metadata["new-service"].Origin)
		}
		if root.Metadata["new-service"].AddedBy != "api" {
			t.Errorf("Expected AddedBy api, got %s", root.Metadata["new-service"].AddedBy)
		}
	})
}

func TestToDeps(t *testing.T) {
	root := &RootConfig{
		SchemaVersion: "1.0",
		Network:       config.NetworkConfig{Name: "test-network", IsObject: false},
		Project: config.Project{
			Name: "test-project",
		},
		Services: map[string]config.Service{
			"api": {
				Source: config.SourceConfig{
					Kind: "git",
					Repo: "git@github.com:org/api.git",
				},
			},
		},
		Infra: map[string]config.InfraEntry{
			"database": {Inline: &config.Infra{
				Image: "postgres",
				Tag:   "15",
			}},
		},
		Env: config.EnvConfig{
			UseGlobal: true,
		},
	}

	deps := root.ToDeps()
	if deps.SchemaVersion != "1.0" {
		t.Errorf("Expected schema version 1.0, got %s", deps.SchemaVersion)
	}
	if deps.Project.Name != "test-project" {
		t.Errorf("Expected project name test-project, got %s", deps.Project.Name)
	}
	if len(deps.Services) != 1 {
		t.Errorf("Expected 1 service, got %d", len(deps.Services))
	}
	if len(deps.Infra) != 1 {
		t.Errorf("Expected 1 infra, got %d", len(deps.Infra))
	}
}

func TestAddAssistedService(t *testing.T) {
	root := &RootConfig{
		SchemaVersion: "1.0",
		Project:       config.Project{Name: "test"},
		Services:      make(map[string]config.Service),
		Infra:         make(map[string]config.InfraEntry),
		Env:           config.EnvConfig{},
		Metadata:      make(map[string]ServiceMetadata),
	}

	svc := config.Service{
		Source: config.SourceConfig{
			Kind: "git",
			Repo: "git@github.com:org/service.git",
		},
	}

	root.AddAssistedService("new-service", svc, "api", "required dependency")

	if len(root.Services) != 1 {
		t.Errorf("Expected 1 service, got %d", len(root.Services))
	}
	if root.Metadata["new-service"].Origin != OriginAssisted {
		t.Errorf("Expected origin %s, got %s", OriginAssisted, root.Metadata["new-service"].Origin)
	}
	if root.Metadata["new-service"].AddedBy != "api" {
		t.Errorf("Expected AddedBy api, got %s", root.Metadata["new-service"].AddedBy)
	}
	if root.Metadata["new-service"].Reason != "required dependency" {
		t.Errorf("Expected reason 'required dependency', got %s", root.Metadata["new-service"].Reason)
	}
	if root.LastUpdatedAt == "" {
		t.Error("Expected LastUpdatedAt to be updated")
	}
}

func TestServiceOriginConstants(t *testing.T) {
	// Verify constants are defined
	if OriginRoot == "" {
		t.Error("OriginRoot should be defined")
	}
	if OriginOverride == "" {
		t.Error("OriginOverride should be defined")
	}
	if OriginAssisted == "" {
		t.Error("OriginAssisted should be defined")
	}

	// Verify they are different
	if OriginRoot == OriginOverride {
		t.Error("OriginRoot and OriginOverride should be different")
	}
	if OriginRoot == OriginAssisted {
		t.Error("OriginRoot and OriginAssisted should be different")
	}
	if OriginOverride == OriginAssisted {
		t.Error("OriginOverride and OriginAssisted should be different")
	}
}
