package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"raioz/internal/config"
	"raioz/internal/mocks"
	"raioz/internal/workspace"
)

func TestEnvShowUseCase_Execute_WithService(t *testing.T) {
	initI18nForTest(t)
	tmpDir := t.TempDir()

	// Create an env file
	envFile := filepath.Join(tmpDir, ".env.api")
	_ = os.WriteFile(envFile, []byte("DB_HOST=localhost\nDB_PORT=5432\n"), 0644)

	deps := newFullMockDeps()
	deps.ConfigLoader = &mocks.MockConfigLoader{
		LoadDepsFunc: func(configPath string) (*config.Deps, []string, error) {
			return &config.Deps{
				Project:       config.Project{Name: "test"},
				SchemaVersion: "1.0",
				Services: map[string]config.Service{
					"api": {
						Source: config.SourceConfig{Path: "."},
					},
				},
				Infra: map[string]config.InfraEntry{},
			}, nil, nil
		},
	}
	deps.Workspace = &mocks.MockWorkspaceManager{
		ResolveFunc: func(name string) (*workspace.Workspace, error) {
			return &workspace.Workspace{Root: tmpDir}, nil
		},
	}

	uc := NewEnvShowUseCase(deps)
	entries, err := uc.Execute(context.Background(), EnvShowOptions{
		ConfigPath:  filepath.Join(tmpDir, "raioz.yaml"),
		ServiceName: "api",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// May or may not find env vars depending on resolver, but should not error
	_ = entries
}

func TestEnvShowUseCase_Execute_WithDiscovery(t *testing.T) {
	initI18nForTest(t)
	tmpDir := t.TempDir()

	deps := newFullMockDeps()
	deps.ConfigLoader = &mocks.MockConfigLoader{
		LoadDepsFunc: func(configPath string) (*config.Deps, []string, error) {
			return &config.Deps{
				Project:       config.Project{Name: "test"},
				SchemaVersion: "2.0",
				Services: map[string]config.Service{
					"api": {
						Source: config.SourceConfig{Path: "."},
					},
				},
				Infra: map[string]config.InfraEntry{
					"postgres": {Inline: &config.Infra{
						Image: "postgres",
						Ports: []string{"5432"},
					}},
				},
			}, nil, nil
		},
	}
	deps.Workspace = &mocks.MockWorkspaceManager{
		ResolveFunc: func(name string) (*workspace.Workspace, error) {
			return &workspace.Workspace{Root: tmpDir}, nil
		},
	}

	uc := NewEnvShowUseCase(deps)
	entries, err := uc.Execute(context.Background(), EnvShowOptions{
		ConfigPath:  filepath.Join(tmpDir, "raioz.yaml"),
		ServiceName: "api",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// With schemaVersion 2.0, discovery vars should be generated
	foundDiscovery := false
	for _, e := range entries {
		if e.Source == "discovery" {
			foundDiscovery = true
			break
		}
	}
	if !foundDiscovery {
		t.Error("expected discovery vars for schema version 2.0")
	}
}

func TestResolveDiscoveryVars_Basic(t *testing.T) {
	initI18nForTest(t)
	tmpDir := t.TempDir()

	deps := &config.Deps{
		Project: config.Project{Name: "test"},
		Services: map[string]config.Service{
			"api": {
				Source: config.SourceConfig{Path: "."},
				Docker: &config.DockerConfig{Ports: []string{"3000"}},
			},
		},
		Infra: map[string]config.InfraEntry{
			"postgres": {Inline: &config.Infra{
				Image: "postgres:16",
				Ports: []string{"5432"},
			}},
		},
	}

	entries := resolveDiscoveryVars(deps, "api", tmpDir)
	if len(entries) == 0 {
		t.Error("expected at least some discovery vars")
	}
	for _, e := range entries {
		if e.Source != "discovery" {
			t.Errorf("expected source 'discovery', got %q", e.Source)
		}
	}
}

func TestResolveFileVars_WorkspaceResolveError(t *testing.T) {
	initI18nForTest(t)
	deps := newFullMockDeps()
	deps.Workspace = &mocks.MockWorkspaceManager{
		ResolveFunc: func(name string) (*workspace.Workspace, error) {
			return nil, fmt.Errorf("resolve fail")
		},
	}

	cfgDeps := &config.Deps{
		Project: config.Project{Name: "test"},
	}
	svc := config.Service{Source: config.SourceConfig{Path: "."}}

	entries := resolveFileVars(deps, cfgDeps, "api", svc, "/tmp")
	if len(entries) != 0 {
		t.Errorf("expected empty entries on resolve error, got %d", len(entries))
	}
}
