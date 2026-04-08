package checkcase

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"raioz/internal/config"
	"raioz/internal/i18n"
	"raioz/internal/mocks"
	"raioz/internal/state"
	"raioz/internal/workspace"
)

func initI18n(t *testing.T) {
	t.Helper()
	os.Setenv("RAIOZ_LANG", "en")
	t.Cleanup(func() { os.Unsetenv("RAIOZ_LANG") })
	i18n.Init("en")
}

func testDeps() *config.Deps {
	return &config.Deps{
		SchemaVersion: "1.0",
		Network:       config.NetworkConfig{Name: "test-net"},
		Project:       config.Project{Name: "test-project"},
		Services: map[string]config.Service{
			"api": {
				Source: config.SourceConfig{Kind: "image", Image: "org/api", Tag: "latest"},
				Docker: &config.DockerConfig{Mode: "prod", Ports: []string{"3000:3000"}},
			},
		},
		Infra: map[string]config.InfraEntry{},
		Env:   config.EnvConfig{UseGlobal: true, Files: []string{"global"}},
	}
}

func TestNewUseCase(t *testing.T) {
	uc := NewUseCase(&Dependencies{})
	if uc == nil {
		t.Fatal("NewUseCase should return non-nil")
	}
}

func TestResolveWorkspaceFromConfig(t *testing.T) {
	initI18n(t)

	cfgDeps := testDeps()
	ws := &workspace.Workspace{Root: "/tmp/test", ServicesDir: "/tmp/test/services"}

	uc := NewUseCase(&Dependencies{
		ConfigLoader: &mocks.MockConfigLoader{
			LoadDepsFunc: func(path string) (*config.Deps, []string, error) {
				return cfgDeps, nil, nil
			},
		},
		Workspace: &mocks.MockWorkspaceManager{
			ResolveFunc: func(name string) (*workspace.Workspace, error) {
				return ws, nil
			},
		},
	})

	name, resolved, err := uc.resolveWorkspace(Options{ConfigPath: ".raioz.json"})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if name != "test-project" {
		t.Errorf("name = %s, want test-project", name)
	}
	if resolved == nil {
		t.Error("workspace should not be nil")
	}
}

func TestResolveWorkspaceNoConfig(t *testing.T) {
	initI18n(t)

	uc := NewUseCase(&Dependencies{
		ConfigLoader: &mocks.MockConfigLoader{
			LoadDepsFunc: func(path string) (*config.Deps, []string, error) {
				return nil, nil, nil
			},
		},
	})

	_, _, err := uc.resolveWorkspace(Options{ConfigPath: "nonexistent.json"})
	if err == nil {
		t.Error("expected error when config cannot be loaded")
	}
}

func TestResolveWorkspaceWithProjectName(t *testing.T) {
	initI18n(t)

	ws := &workspace.Workspace{Root: "/tmp/test"}

	uc := NewUseCase(&Dependencies{
		ConfigLoader: &mocks.MockConfigLoader{
			LoadDepsFunc: func(path string) (*config.Deps, []string, error) {
				return nil, nil, nil
			},
		},
		Workspace: &mocks.MockWorkspaceManager{
			ResolveFunc: func(name string) (*workspace.Workspace, error) {
				return ws, nil
			},
		},
	})

	name, _, err := uc.resolveWorkspace(Options{ProjectName: "my-proj", ConfigPath: ".raioz.json"})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if name != "my-proj" {
		t.Errorf("name = %s, want my-proj", name)
	}
}

func TestLoadConfig(t *testing.T) {
	initI18n(t)

	cfgDeps := testDeps()
	uc := NewUseCase(&Dependencies{
		ConfigLoader: &mocks.MockConfigLoader{
			LoadDepsFunc: func(path string) (*config.Deps, []string, error) {
				return cfgDeps, nil, nil
			},
		},
	})

	loaded, err := uc.loadConfig(".raioz.json")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if loaded.Project.Name != "test-project" {
		t.Errorf("Project.Name = %s, want test-project", loaded.Project.Name)
	}
}

func TestLoadConfigError(t *testing.T) {
	initI18n(t)

	uc := NewUseCase(&Dependencies{
		ConfigLoader: &mocks.MockConfigLoader{
			LoadDepsFunc: func(path string) (*config.Deps, []string, error) {
				return nil, nil, os.ErrNotExist
			},
		},
	})

	_, err := uc.loadConfig("nonexistent.json")
	if err == nil {
		t.Error("expected error when config loading fails")
	}
}

func TestValidateConfigValid(t *testing.T) {
	initI18n(t)

	uc := NewUseCase(&Dependencies{})
	deps := testDeps()

	errs := uc.validateConfig(deps)
	if len(errs) > 0 {
		t.Errorf("expected no validation errors, got: %v", errs)
	}
}

func TestValidateConfigInvalid(t *testing.T) {
	initI18n(t)

	uc := NewUseCase(&Dependencies{})
	deps := &config.Deps{
		SchemaVersion: "invalid",
		Project:       config.Project{Name: ""},
	}

	errs := uc.validateConfig(deps)
	if len(errs) == 0 {
		t.Error("expected validation errors for invalid config")
	}
}

func TestCheckAlignmentNoIssues(t *testing.T) {
	initI18n(t)

	tmpDir := t.TempDir()
	ws := &workspace.Workspace{Root: tmpDir, ServicesDir: filepath.Join(tmpDir, "services")}

	deps := testDeps()
	state.Save(ws, deps)

	uc := NewUseCase(&Dependencies{})
	issues, err := uc.checkAlignment(ws, deps)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(issues) != 0 {
		t.Errorf("expected no issues, got %d", len(issues))
	}
}

func TestExecuteNoState(t *testing.T) {
	initI18n(t)

	cfgDeps := testDeps()
	ws := &workspace.Workspace{Root: "/tmp/test"}

	uc := NewUseCase(&Dependencies{
		ConfigLoader: &mocks.MockConfigLoader{
			LoadDepsFunc: func(path string) (*config.Deps, []string, error) {
				return cfgDeps, nil, nil
			},
		},
		Workspace: &mocks.MockWorkspaceManager{
			ResolveFunc: func(name string) (*workspace.Workspace, error) {
				return ws, nil
			},
		},
		StateManager: &mocks.MockStateManager{
			ExistsFunc: func(ws *workspace.Workspace) bool {
				return false
			},
		},
	})

	result, err := uc.Execute(context.Background(), Options{ConfigPath: ".raioz.json"})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !result.NoState {
		t.Error("expected NoState = true")
	}
	if !result.ConfigValid {
		t.Error("config should be valid")
	}
}

func TestExecuteWithConfigError(t *testing.T) {
	initI18n(t)

	uc := NewUseCase(&Dependencies{
		ConfigLoader: &mocks.MockConfigLoader{
			LoadDepsFunc: func(path string) (*config.Deps, []string, error) {
				return nil, nil, nil
			},
		},
	})

	_, err := uc.Execute(context.Background(), Options{ConfigPath: "bad.json"})
	if err == nil {
		t.Error("expected error when config cannot be resolved")
	}
}

func TestExecuteWithAlignedState(t *testing.T) {
	initI18n(t)

	tmpDir := t.TempDir()
	ws := &workspace.Workspace{Root: tmpDir, ServicesDir: filepath.Join(tmpDir, "services")}
	cfgDeps := testDeps()
	state.Save(ws, cfgDeps)

	uc := NewUseCase(&Dependencies{
		ConfigLoader: &mocks.MockConfigLoader{
			LoadDepsFunc: func(path string) (*config.Deps, []string, error) {
				return cfgDeps, nil, nil
			},
		},
		Workspace: &mocks.MockWorkspaceManager{
			ResolveFunc: func(name string) (*workspace.Workspace, error) {
				return ws, nil
			},
		},
		StateManager: &mocks.MockStateManager{
			ExistsFunc: func(ws *workspace.Workspace) bool {
				return true
			},
		},
	})

	result, err := uc.Execute(context.Background(), Options{ConfigPath: ".raioz.json"})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if result.HasIssues {
		t.Error("expected no issues for aligned state")
	}
	if result.ConfigValid != true {
		t.Error("config should be valid")
	}
}

func TestExecuteWithInvalidConfig(t *testing.T) {
	initI18n(t)

	ws := &workspace.Workspace{Root: "/tmp/test"}
	badDeps := &config.Deps{
		SchemaVersion: "invalid",
		Project:       config.Project{Name: ""},
	}

	uc := NewUseCase(&Dependencies{
		ConfigLoader: &mocks.MockConfigLoader{
			LoadDepsFunc: func(path string) (*config.Deps, []string, error) {
				return badDeps, nil, nil
			},
		},
		Workspace: &mocks.MockWorkspaceManager{
			ResolveFunc: func(name string) (*workspace.Workspace, error) {
				return ws, nil
			},
		},
		StateManager: &mocks.MockStateManager{
			ExistsFunc: func(ws *workspace.Workspace) bool {
				return false
			},
		},
	})

	result, err := uc.Execute(context.Background(), Options{ConfigPath: ".raioz.json"})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if result.ConfigValid {
		t.Error("config should be invalid")
	}
	if len(result.ValidationErrors) == 0 {
		t.Error("expected validation errors")
	}
	if !result.HasIssues {
		t.Error("HasIssues should be true for invalid config")
	}
}
