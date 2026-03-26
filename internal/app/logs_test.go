package app

import (
	"context"
	"fmt"
	"testing"

	"raioz/internal/config"
	"raioz/internal/domain/interfaces"
	"raioz/internal/mocks"
	"raioz/internal/workspace"
)

func newTestDepsForLogs(t *testing.T) (*Dependencies, *mocks.MockConfigLoader, *mocks.MockWorkspaceManager, *mocks.MockStateManager, *mocks.MockDockerRunner) {
	t.Helper()

	tmpDir := t.TempDir()

	configLoader := &mocks.MockConfigLoader{}
	wsMgr := &mocks.MockWorkspaceManager{
		ResolveFunc: func(projectName string) (*workspace.Workspace, error) {
			return &workspace.Workspace{Root: tmpDir}, nil
		},
		GetComposePathFunc: func(ws *workspace.Workspace) string {
			return tmpDir + "/docker-compose.generated.yml"
		},
		GetBaseDirFunc: func() (string, error) {
			return tmpDir, nil
		},
	}
	stateMgr := &mocks.MockStateManager{
		ExistsFunc: func(ws *workspace.Workspace) bool { return true },
		LoadFunc: func(ws *workspace.Workspace) (*config.Deps, error) {
			return &config.Deps{Project: config.Project{Name: "test-project"}}, nil
		},
	}
	dockerRunner := &mocks.MockDockerRunner{}

	deps := &Dependencies{
		ConfigLoader: configLoader,
		Workspace:    wsMgr,
		StateManager: stateMgr,
		DockerRunner: dockerRunner,
		Validator:    &mocks.MockValidator{},
		GitRepository: &mocks.MockGitRepository{},
		LockManager:  &mocks.MockLockManager{},
		HostRunner:   &mocks.MockHostRunner{},
		EnvManager:   &mocks.MockEnvManager{},
	}

	return deps, configLoader, wsMgr, stateMgr, dockerRunner
}

func TestLogsUseCase_Execute_NoProject(t *testing.T) {
	deps, configLoader, _, _, _ := newTestDepsForLogs(t)

	// ConfigLoader returns nil (no config found)
	configLoader.LoadDepsFunc = func(configPath string) (*config.Deps, []string, error) {
		return nil, nil, fmt.Errorf("not found")
	}

	uc := NewLogsUseCase(deps)
	err := uc.Execute(context.Background(), LogsOptions{})

	if err == nil {
		t.Fatal("expected error when no project name and no config, got nil")
	}
	if got := err.Error(); got != "could not determine project name. Please provide --file or --project flag" {
		t.Errorf("unexpected error message: %s", got)
	}
}

func TestLogsUseCase_Execute_NotRunning(t *testing.T) {
	deps, configLoader, _, stateMgr, _ := newTestDepsForLogs(t)

	configLoader.LoadDepsFunc = func(configPath string) (*config.Deps, []string, error) {
		return &config.Deps{Project: config.Project{Name: "test-project"}}, nil, nil
	}

	// State does not exist (project not running)
	stateMgr.ExistsFunc = func(ws *workspace.Workspace) bool { return false }

	uc := NewLogsUseCase(deps)
	err := uc.Execute(context.Background(), LogsOptions{})

	if err == nil {
		t.Fatal("expected error when project is not running, got nil")
	}
	if got := err.Error(); got != "project is not running (no state file found)" {
		t.Errorf("unexpected error message: %s", got)
	}
}

func TestLogsUseCase_Execute_AllServices(t *testing.T) {
	deps, configLoader, _, _, dockerRunner := newTestDepsForLogs(t)

	configLoader.LoadDepsFunc = func(configPath string) (*config.Deps, []string, error) {
		return &config.Deps{Project: config.Project{Name: "test-project"}}, nil, nil
	}

	var viewLogsCalled bool
	dockerRunner.GetAvailableServicesWithContextFunc = func(ctx context.Context, composePath string) ([]string, error) {
		return []string{"api", "web"}, nil
	}
	dockerRunner.ViewLogsWithContextFunc = func(ctx context.Context, composePath string, opts interfaces.LogsOptions) error {
		viewLogsCalled = true
		if len(opts.Services) != 2 {
			t.Errorf("expected 2 services, got %d", len(opts.Services))
		}
		return nil
	}

	uc := NewLogsUseCase(deps)
	err := uc.Execute(context.Background(), LogsOptions{All: true})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !viewLogsCalled {
		t.Error("expected ViewLogsWithContext to be called")
	}
}
