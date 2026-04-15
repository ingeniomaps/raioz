package upcase

import (
	"context"
	stderrors "errors"
	"os"
	"path/filepath"
	"testing"

	"raioz/internal/config"
	"raioz/internal/mocks"
	"raioz/internal/state"
	"raioz/internal/workspace"
)

// localProjectDeps returns minimal Dependencies that won't nil-panic when
// processLocalProject reaches checkAndHandleDuplicateProject (needs ConfigLoader + Workspace).
func localProjectDeps(extras *Dependencies) *Dependencies {
	base := &Dependencies{
		ConfigLoader: &mocks.MockConfigLoader{
			LoadDepsFunc: func(configPath string) (*config.Deps, []string, error) {
				return &config.Deps{Project: config.Project{Name: "myproj"}}, nil, nil
			},
		},
		Workspace: &mocks.MockWorkspaceManager{
			ResolveFunc: func(name string) (*workspace.Workspace, error) {
				return nil, stderrors.New("no workspace")
			},
		},
	}
	if extras != nil {
		if extras.HostRunner != nil {
			base.HostRunner = extras.HostRunner
		}
		if extras.StateManager != nil {
			base.StateManager = extras.StateManager
		}
		if extras.EnvManager != nil {
			base.EnvManager = extras.EnvManager
		}
	}
	return base
}

// --- processLocalProject: local project with commands -------------------------

func TestProcessLocalProjectLocalWithUpCommand(t *testing.T) {
	initI18nUp(t)

	// Create a local project directory outside RAIOZ_HOME
	raiozHome := t.TempDir()
	t.Setenv("RAIOZ_HOME", raiozHome)

	projectDir := t.TempDir()
	configPath := filepath.Join(projectDir, ".raioz.json")

	uc := NewUseCase(localProjectDeps(&Dependencies{
		HostRunner: &mocks.MockHostRunner{
			DetectComposePathFunc: func(servicePath, command, explicit string) string {
				return ""
			},
		},
		StateManager: &mocks.MockStateManager{
			UpdateProjectStateFunc: func(name string, ps *state.ProjectState) error {
				return nil
			},
		},
	}))

	deps := &config.Deps{
		Project: config.Project{
			Name: "myproj",
			Commands: &config.ProjectCommands{
				Dev: &config.EnvironmentCommands{Up: "true"},
			},
		},
	}

	err := uc.processLocalProject(context.Background(), configPath, deps, "up", nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestProcessLocalProjectLocalWithDownCommand(t *testing.T) {
	initI18nUp(t)

	raiozHome := t.TempDir()
	t.Setenv("RAIOZ_HOME", raiozHome)

	projectDir := t.TempDir()
	configPath := filepath.Join(projectDir, ".raioz.json")

	uc := NewUseCase(localProjectDeps(nil))

	deps := &config.Deps{
		Project: config.Project{
			Name: "myproj",
			Commands: &config.ProjectCommands{
				Dev: &config.EnvironmentCommands{Down: "true"},
			},
		},
	}

	err := uc.processLocalProject(context.Background(), configPath, deps, "down", nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestProcessLocalProjectLocalNoCommand(t *testing.T) {
	initI18nUp(t)

	raiozHome := t.TempDir()
	t.Setenv("RAIOZ_HOME", raiozHome)

	projectDir := t.TempDir()
	configPath := filepath.Join(projectDir, ".raioz.json")

	uc := NewUseCase(localProjectDeps(nil))

	deps := &config.Deps{
		Project: config.Project{Name: "myproj"},
	}

	// No commands defined → no-op
	err := uc.processLocalProject(context.Background(), configPath, deps, "up", nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestProcessLocalProjectNotLocalWithUpCommand(t *testing.T) {
	initI18nUp(t)

	raiozHome := t.TempDir()
	t.Setenv("RAIOZ_HOME", raiozHome)

	// Create inside workspaces → not local
	wsDir := filepath.Join(raiozHome, "workspaces", "x")
	if err := os.MkdirAll(wsDir, 0755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(wsDir, ".raioz.json")

	uc := NewUseCase(&Dependencies{
		HostRunner: &mocks.MockHostRunner{
			DetectComposePathFunc: func(servicePath, command, explicit string) string {
				return ""
			},
		},
		StateManager: &mocks.MockStateManager{
			UpdateProjectStateFunc: func(name string, ps *state.ProjectState) error {
				return nil
			},
		},
	})

	deps := &config.Deps{
		Project: config.Project{
			Name: "myproj",
			Commands: &config.ProjectCommands{
				Dev: &config.EnvironmentCommands{Up: "true"},
			},
		},
	}

	err := uc.processLocalProject(context.Background(), configPath, deps, "up", nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestProcessLocalProjectFailingCommand(t *testing.T) {
	initI18nUp(t)

	raiozHome := t.TempDir()
	t.Setenv("RAIOZ_HOME", raiozHome)

	projectDir := t.TempDir()
	configPath := filepath.Join(projectDir, ".raioz.json")

	uc := NewUseCase(localProjectDeps(nil))

	deps := &config.Deps{
		Project: config.Project{
			Name: "myproj",
			Commands: &config.ProjectCommands{
				Dev: &config.EnvironmentCommands{Up: "false"},
			},
		},
	}

	err := uc.processLocalProject(context.Background(), configPath, deps, "up", nil)
	if err == nil {
		t.Error("expected error from failing command")
	}
}

func TestProcessLocalProjectWithHealthCheckAlreadyHealthy(t *testing.T) {
	initI18nUp(t)

	raiozHome := t.TempDir()
	t.Setenv("RAIOZ_HOME", raiozHome)

	projectDir := t.TempDir()
	configPath := filepath.Join(projectDir, ".raioz.json")

	uc := NewUseCase(localProjectDeps(nil))

	deps := &config.Deps{
		Project: config.Project{
			Name: "myproj",
			Commands: &config.ProjectCommands{
				Health: "true",
				Dev:    &config.EnvironmentCommands{Up: "echo should-not-run"},
			},
		},
	}

	// Health returns true → up should be skipped
	err := uc.processLocalProject(context.Background(), configPath, deps, "up", nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestProcessLocalProjectDownNotHealthy(t *testing.T) {
	initI18nUp(t)

	raiozHome := t.TempDir()
	t.Setenv("RAIOZ_HOME", raiozHome)

	projectDir := t.TempDir()
	configPath := filepath.Join(projectDir, ".raioz.json")

	uc := NewUseCase(localProjectDeps(nil))

	deps := &config.Deps{
		Project: config.Project{
			Name: "myproj",
			Commands: &config.ProjectCommands{
				Health: "false", // Not healthy
				Dev:    &config.EnvironmentCommands{Down: "echo should-not-run"},
			},
		},
	}

	// Health returns false → down should be skipped (not running)
	err := uc.processLocalProject(context.Background(), configPath, deps, "down", nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestProcessLocalProjectWithProdMode(t *testing.T) {
	initI18nUp(t)

	raiozHome := t.TempDir()
	t.Setenv("RAIOZ_HOME", raiozHome)

	projectDir := t.TempDir()
	configPath := filepath.Join(projectDir, ".raioz.json")

	uc := NewUseCase(localProjectDeps(&Dependencies{
		HostRunner: &mocks.MockHostRunner{
			DetectComposePathFunc: func(servicePath, command, explicit string) string {
				return ""
			},
		},
		StateManager: &mocks.MockStateManager{
			UpdateProjectStateFunc: func(name string, ps *state.ProjectState) error {
				return nil
			},
		},
	}))

	deps := &config.Deps{
		Project: config.Project{
			Name: "myproj",
			Commands: &config.ProjectCommands{
				Prod: &config.EnvironmentCommands{Up: "true"},
			},
		},
		Services: map[string]config.Service{
			"api": {Docker: &config.DockerConfig{Mode: "prod"}},
		},
	}

	err := uc.processLocalProject(context.Background(), configPath, deps, "up", nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestProcessLocalProjectWithWorkspace(t *testing.T) {
	initI18nUp(t)

	raiozHome := t.TempDir()
	t.Setenv("RAIOZ_HOME", raiozHome)

	projectDir := t.TempDir()
	configPath := filepath.Join(projectDir, ".raioz.json")

	ws := &workspace.Workspace{Root: t.TempDir()}

	uc := NewUseCase(localProjectDeps(&Dependencies{
		HostRunner: &mocks.MockHostRunner{
			DetectComposePathFunc: func(servicePath, command, explicit string) string {
				return ""
			},
		},
		StateManager: &mocks.MockStateManager{
			UpdateProjectStateFunc: func(name string, ps *state.ProjectState) error {
				return nil
			},
			SaveFunc: func(ws2 *workspace.Workspace, deps *config.Deps) error {
				return nil
			},
		},
		EnvManager: &mocks.MockEnvManager{
			ResolveProjectEnvFunc: func(ws2 *workspace.Workspace, deps *config.Deps, projectDir string) (string, error) {
				return "", nil
			},
			GenerateEnvFromTemplateFunc: func(
				ws2 *workspace.Workspace, d *config.Deps, name, path string,
				svc config.Service, projEnv, projDir string,
			) error {
				return nil
			},
		},
	}))

	deps := &config.Deps{
		Project: config.Project{
			Name: "myproj",
			Commands: &config.ProjectCommands{
				Dev: &config.EnvironmentCommands{Up: "true"},
			},
		},
	}

	// Pass workspace as interface{}
	err := uc.processLocalProject(context.Background(), configPath, deps, "up", ws)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
