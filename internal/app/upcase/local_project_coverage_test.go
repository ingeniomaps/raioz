package upcase

import (
	"context"
	stderrors "errors"
	"os"
	"path/filepath"
	"testing"

	"raioz/internal/domain/models"
	"raioz/internal/mocks"
	"raioz/internal/workspace"
)

// localProjectDeps returns minimal Dependencies that won't nil-panic when
// processLocalProject reaches checkAndHandleDuplicateProject (needs ConfigLoader + Workspace).
func localProjectDeps(extras *Dependencies) *Dependencies {
	base := &Dependencies{
		ConfigLoader: &mocks.MockConfigLoader{
			LoadDepsFunc: func(configPath string) (*models.Deps, []string, error) {
				return &models.Deps{Project: models.Project{Name: "myproj"}}, nil, nil
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
			UpdateProjectStateFunc: func(name string, ps *models.ProjectState) error {
				return nil
			},
		},
	}))

	deps := &models.Deps{
		Project: models.Project{
			Name: "myproj",
			Commands: &models.ProjectCommands{
				Dev: &models.EnvironmentCommands{Up: "true"},
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

	deps := &models.Deps{
		Project: models.Project{
			Name: "myproj",
			Commands: &models.ProjectCommands{
				Dev: &models.EnvironmentCommands{Down: "true"},
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

	deps := &models.Deps{
		Project: models.Project{Name: "myproj"},
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
			UpdateProjectStateFunc: func(name string, ps *models.ProjectState) error {
				return nil
			},
		},
	})

	deps := &models.Deps{
		Project: models.Project{
			Name: "myproj",
			Commands: &models.ProjectCommands{
				Dev: &models.EnvironmentCommands{Up: "true"},
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

	deps := &models.Deps{
		Project: models.Project{
			Name: "myproj",
			Commands: &models.ProjectCommands{
				Dev: &models.EnvironmentCommands{Up: "false"},
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

	deps := &models.Deps{
		Project: models.Project{
			Name: "myproj",
			Commands: &models.ProjectCommands{
				Health: "true",
				Dev:    &models.EnvironmentCommands{Up: "echo should-not-run"},
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

	deps := &models.Deps{
		Project: models.Project{
			Name: "myproj",
			Commands: &models.ProjectCommands{
				Health: "false", // Not healthy
				Dev:    &models.EnvironmentCommands{Down: "echo should-not-run"},
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
			UpdateProjectStateFunc: func(name string, ps *models.ProjectState) error {
				return nil
			},
		},
	}))

	deps := &models.Deps{
		Project: models.Project{
			Name: "myproj",
			Commands: &models.ProjectCommands{
				Prod: &models.EnvironmentCommands{Up: "true"},
			},
		},
		Services: map[string]models.Service{
			"api": {Docker: &models.DockerConfig{Mode: "prod"}},
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
			UpdateProjectStateFunc: func(name string, ps *models.ProjectState) error {
				return nil
			},
			SaveFunc: func(ws2 *workspace.Workspace, deps *models.Deps) error {
				return nil
			},
		},
		EnvManager: &mocks.MockEnvManager{
			ResolveProjectEnvFunc: func(ws2 *workspace.Workspace, deps *models.Deps, projectDir string) (string, error) {
				return "", nil
			},
			GenerateEnvFromTemplateFunc: func(
				ws2 *workspace.Workspace, d *models.Deps, name, path string,
				svc models.Service, projEnv, projDir string,
			) error {
				return nil
			},
		},
	}))

	deps := &models.Deps{
		Project: models.Project{
			Name: "myproj",
			Commands: &models.ProjectCommands{
				Dev: &models.EnvironmentCommands{Up: "true"},
			},
		},
	}

	// Pass workspace as interface{}
	err := uc.processLocalProject(context.Background(), configPath, deps, "up", ws)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
