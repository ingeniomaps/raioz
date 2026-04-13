package app

import (
	"context"
	"fmt"
	"testing"

	"raioz/internal/config"
	"raioz/internal/mocks"
	"raioz/internal/workspace"
)

func newTestDepsForRestart(t *testing.T) (*Dependencies, *mocks.MockConfigLoader,
	*mocks.MockWorkspaceManager, *mocks.MockStateManager, *mocks.MockDockerRunner,
	*mocks.MockHostRunner,
) {
	t.Helper()
	tmpDir := t.TempDir()

	configLoader := &mocks.MockConfigLoader{}
	wsMgr := &mocks.MockWorkspaceManager{
		ResolveFunc: func(name string) (*workspace.Workspace, error) {
			return &workspace.Workspace{Root: tmpDir}, nil
		},
		GetComposePathFunc: func(ws *workspace.Workspace) string {
			return tmpDir + "/docker-compose.generated.yml"
		},
	}
	stateMgr := &mocks.MockStateManager{
		ExistsFunc: func(ws *workspace.Workspace) bool { return true },
		LoadFunc: func(ws *workspace.Workspace) (*config.Deps, error) {
			return &config.Deps{Project: config.Project{Name: "test-project"}}, nil
		},
	}
	dockerRunner := &mocks.MockDockerRunner{}
	hostRunner := &mocks.MockHostRunner{}

	deps := &Dependencies{
		ConfigLoader:  configLoader,
		Workspace:     wsMgr,
		StateManager:  stateMgr,
		DockerRunner:  dockerRunner,
		Validator:     &mocks.MockValidator{},
		GitRepository: &mocks.MockGitRepository{},
		LockManager:   &mocks.MockLockManager{},
		HostRunner:    hostRunner,
		EnvManager:    &mocks.MockEnvManager{},
	}
	return deps, configLoader, wsMgr, stateMgr, dockerRunner, hostRunner
}

func TestNewRestartUseCase(t *testing.T) {
	uc := NewRestartUseCase(newFullMockDeps())
	if uc == nil {
		t.Fatal("expected non-nil RestartUseCase")
	}
	if uc.Out == nil {
		t.Error("expected non-nil Out writer")
	}
}

func TestRestartUseCase_Execute_NoProject(t *testing.T) {
	initI18nForTest(t)
	deps, configLoader, _, _, _, _ := newTestDepsForRestart(t)
	configLoader.LoadDepsFunc = func(configPath string) (*config.Deps, []string, error) {
		return nil, nil, fmt.Errorf("nope")
	}
	uc := NewRestartUseCase(deps)
	err := uc.Execute(context.Background(), RestartOptions{})
	if err == nil {
		t.Fatal("expected error for no project, got nil")
	}
}

func TestRestartUseCase_Execute_WorkspaceResolveFails(t *testing.T) {
	initI18nForTest(t)
	deps, configLoader, wsMgr, _, _, _ := newTestDepsForRestart(t)
	configLoader.LoadDepsFunc = func(configPath string) (*config.Deps, []string, error) {
		return &config.Deps{Project: config.Project{Name: "proj"}}, nil, nil
	}
	wsMgr.ResolveFunc = func(name string) (*workspace.Workspace, error) {
		return nil, fmt.Errorf("not found")
	}
	uc := NewRestartUseCase(deps)
	err := uc.Execute(context.Background(), RestartOptions{})
	if err == nil {
		t.Fatal("expected error for workspace resolve failure, got nil")
	}
}

func TestRestartUseCase_Execute_NotRunning(t *testing.T) {
	initI18nForTest(t)
	deps, configLoader, _, stateMgr, _, _ := newTestDepsForRestart(t)
	configLoader.LoadDepsFunc = func(configPath string) (*config.Deps, []string, error) {
		return &config.Deps{Project: config.Project{Name: "proj"}}, nil, nil
	}
	stateMgr.ExistsFunc = func(ws *workspace.Workspace) bool { return false }
	uc := NewRestartUseCase(deps)
	err := uc.Execute(context.Background(), RestartOptions{})
	if err == nil {
		t.Fatal("expected error for not running, got nil")
	}
}

func TestRestartUseCase_Execute_StateLoadFails(t *testing.T) {
	initI18nForTest(t)
	deps, configLoader, _, stateMgr, _, _ := newTestDepsForRestart(t)
	configLoader.LoadDepsFunc = func(configPath string) (*config.Deps, []string, error) {
		return &config.Deps{Project: config.Project{Name: "proj"}}, nil, nil
	}
	stateMgr.LoadFunc = func(ws *workspace.Workspace) (*config.Deps, error) {
		return nil, fmt.Errorf("corrupt")
	}
	uc := NewRestartUseCase(deps)
	err := uc.Execute(context.Background(), RestartOptions{})
	if err == nil {
		t.Fatal("expected error for state load failure, got nil")
	}
}

func TestRestartUseCase_Execute_NoServicesRequested(t *testing.T) {
	initI18nForTest(t)
	deps, configLoader, _, _, dockerRunner, _ := newTestDepsForRestart(t)
	configLoader.LoadDepsFunc = func(configPath string) (*config.Deps, []string, error) {
		return &config.Deps{Project: config.Project{Name: "proj"}}, nil, nil
	}
	dockerRunner.GetAvailableServicesWithContextFunc = func(ctx context.Context, composePath string) ([]string, error) {
		return []string{"api"}, nil
	}
	uc := NewRestartUseCase(deps)
	// No Services specified and not All -> empty slice -> should return error "no services"
	err := uc.Execute(context.Background(), RestartOptions{})
	if err == nil {
		t.Fatal("expected error for no services to restart, got nil")
	}
}

func TestRestartUseCase_Execute_AllServices(t *testing.T) {
	initI18nForTest(t)
	deps, configLoader, _, _, dockerRunner, _ := newTestDepsForRestart(t)
	configLoader.LoadDepsFunc = func(configPath string) (*config.Deps, []string, error) {
		return &config.Deps{Project: config.Project{Name: "proj"}}, nil, nil
	}
	dockerRunner.GetAvailableServicesWithContextFunc = func(ctx context.Context, composePath string) ([]string, error) {
		return []string{"api", "worker"}, nil
	}
	var restartCalled bool
	dockerRunner.RestartServicesWithContextFunc = func(ctx context.Context, composePath string, serviceNames []string) error {
		restartCalled = true
		return nil
	}
	uc := NewRestartUseCase(deps)
	err := uc.Execute(context.Background(), RestartOptions{All: true, IncludeInfra: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !restartCalled {
		t.Error("expected RestartServicesWithContext to be called")
	}
}

func TestRestartUseCase_Execute_ForceRecreate(t *testing.T) {
	initI18nForTest(t)
	deps, configLoader, _, _, dockerRunner, _ := newTestDepsForRestart(t)
	configLoader.LoadDepsFunc = func(configPath string) (*config.Deps, []string, error) {
		return &config.Deps{Project: config.Project{Name: "proj"}}, nil, nil
	}
	dockerRunner.GetAvailableServicesWithContextFunc = func(ctx context.Context, composePath string) ([]string, error) {
		return []string{"api"}, nil
	}
	var forceCalled bool
	dockerRunner.ForceRecreateServicesWithContextFunc = func(ctx context.Context, composePath string, serviceNames []string) error {
		forceCalled = true
		return nil
	}
	uc := NewRestartUseCase(deps)
	err := uc.Execute(context.Background(), RestartOptions{
		All:           true,
		IncludeInfra:  true,
		ForceRecreate: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !forceCalled {
		t.Error("expected ForceRecreateServicesWithContext to be called")
	}
}

func TestRestartUseCase_Execute_SpecificServices(t *testing.T) {
	initI18nForTest(t)
	deps, configLoader, _, _, dockerRunner, _ := newTestDepsForRestart(t)
	configLoader.LoadDepsFunc = func(configPath string) (*config.Deps, []string, error) {
		return &config.Deps{Project: config.Project{Name: "proj"}}, nil, nil
	}
	dockerRunner.GetAvailableServicesWithContextFunc = func(ctx context.Context, composePath string) ([]string, error) {
		return []string{"api", "worker"}, nil
	}
	var capturedServices []string
	dockerRunner.RestartServicesWithContextFunc = func(ctx context.Context, composePath string, serviceNames []string) error {
		capturedServices = serviceNames
		return nil
	}
	uc := NewRestartUseCase(deps)
	err := uc.Execute(context.Background(), RestartOptions{
		Services: []string{"api"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(capturedServices) != 1 || capturedServices[0] != "api" {
		t.Errorf("expected [api], got %v", capturedServices)
	}
}
