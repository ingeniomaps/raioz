package app

import (
	"context"
	"fmt"
	"testing"

	"raioz/internal/config"
	"raioz/internal/mocks"
	"raioz/internal/workspace"
)

func newTestDepsForClean(t *testing.T) (*Dependencies, *mocks.MockConfigLoader, *mocks.MockWorkspaceManager, *mocks.MockDockerRunner) {
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
		GetStatePathFunc: func(ws *workspace.Workspace) string {
			return tmpDir + "/.state.json"
		},
	}
	dockerRunner := &mocks.MockDockerRunner{}

	deps := &Dependencies{
		ConfigLoader:  configLoader,
		Workspace:     wsMgr,
		StateManager:  &mocks.MockStateManager{},
		DockerRunner:  dockerRunner,
		Validator:     &mocks.MockValidator{},
		GitRepository: &mocks.MockGitRepository{},
		LockManager:   &mocks.MockLockManager{},
		HostRunner:    &mocks.MockHostRunner{},
		EnvManager:    &mocks.MockEnvManager{},
	}

	return deps, configLoader, wsMgr, dockerRunner
}

func TestCleanUseCase_Execute_CleanAll(t *testing.T) {
	deps, _, _, dockerRunner := newTestDepsForClean(t)

	var cleanAllCalled bool
	dockerRunner.CleanAllProjectsWithContextFunc = func(ctx context.Context, baseDir string, dryRun bool) ([]string, error) {
		cleanAllCalled = true
		return []string{"Removed container test-api"}, nil
	}

	uc := NewCleanUseCase(deps)
	err := uc.Execute(context.Background(), CleanOptions{All: true})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cleanAllCalled {
		t.Error("expected CleanAllProjectsWithContext to be called")
	}
}

func TestCleanUseCase_Execute_CleanProject(t *testing.T) {
	deps, configLoader, _, dockerRunner := newTestDepsForClean(t)

	configLoader.LoadDepsFunc = func(configPath string) (*config.Deps, []string, error) {
		return &config.Deps{Project: config.Project{Name: "test-project"}}, nil, nil
	}

	var cleanProjectCalled bool
	dockerRunner.CleanProjectWithContextFunc = func(ctx context.Context, composePath string, dryRun bool) ([]string, error) {
		cleanProjectCalled = true
		return []string{"Removed container test-project-api"}, nil
	}

	uc := NewCleanUseCase(deps)
	err := uc.Execute(context.Background(), CleanOptions{ProjectName: "test-project"})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cleanProjectCalled {
		t.Error("expected CleanProjectWithContext to be called")
	}
}

func TestCleanUseCase_Execute_NoProject(t *testing.T) {
	deps, configLoader, _, _ := newTestDepsForClean(t)

	// No config found and no project name
	configLoader.LoadDepsFunc = func(configPath string) (*config.Deps, []string, error) {
		return nil, nil, fmt.Errorf("not found")
	}

	uc := NewCleanUseCase(deps)
	err := uc.Execute(context.Background(), CleanOptions{})

	if err == nil {
		t.Fatal("expected error when no project name and no config, got nil")
	}
	if err.Error() == "" {
		t.Error("expected non-empty error message")
	}
}
