package app

import (
	"context"
	"fmt"
	"testing"

	"raioz/internal/config"
	"raioz/internal/host"
	"raioz/internal/mocks"
	"raioz/internal/workspace"
)

func newTestDepsForExec(t *testing.T) (*Dependencies, *mocks.MockConfigLoader, *mocks.MockWorkspaceManager, *mocks.MockStateManager, *mocks.MockDockerRunner, *mocks.MockHostRunner) {
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

func TestExecUseCase_Execute_NoConfig(t *testing.T) {
	deps, configLoader, _, _, _, _ := newTestDepsForExec(t)

	configLoader.LoadDepsFunc = func(configPath string) (*config.Deps, []string, error) {
		return nil, nil, fmt.Errorf("not found")
	}

	uc := NewExecUseCase(deps)
	err := uc.Execute(context.Background(), ExecOptions{
		Service: "api",
	})

	if err == nil {
		t.Fatal("expected error when no config found, got nil")
	}
}

func TestExecUseCase_Execute_WorkspaceResolveFails(t *testing.T) {
	deps, configLoader, wsMgr, _, _, _ := newTestDepsForExec(t)

	configLoader.LoadDepsFunc = func(configPath string) (*config.Deps, []string, error) {
		return &config.Deps{Project: config.Project{Name: "test-project"}}, nil, nil
	}
	wsMgr.ResolveFunc = func(projectName string) (*workspace.Workspace, error) {
		return nil, fmt.Errorf("workspace not found")
	}

	uc := NewExecUseCase(deps)
	err := uc.Execute(context.Background(), ExecOptions{
		Service: "api",
	})

	if err == nil {
		t.Fatal("expected error when workspace resolve fails, got nil")
	}
}

func TestExecUseCase_Execute_NotRunning(t *testing.T) {
	deps, configLoader, _, stateMgr, _, _ := newTestDepsForExec(t)

	configLoader.LoadDepsFunc = func(configPath string) (*config.Deps, []string, error) {
		return &config.Deps{Project: config.Project{Name: "test-project"}}, nil, nil
	}
	stateMgr.ExistsFunc = func(ws *workspace.Workspace) bool { return false }

	uc := NewExecUseCase(deps)
	err := uc.Execute(context.Background(), ExecOptions{
		Service: "api",
	})

	if err == nil {
		t.Fatal("expected error when project is not running, got nil")
	}
}

func TestExecUseCase_Execute_StateLoadFails(t *testing.T) {
	deps, configLoader, _, stateMgr, _, _ := newTestDepsForExec(t)

	configLoader.LoadDepsFunc = func(configPath string) (*config.Deps, []string, error) {
		return &config.Deps{Project: config.Project{Name: "test-project"}}, nil, nil
	}
	stateMgr.LoadFunc = func(ws *workspace.Workspace) (*config.Deps, error) {
		return nil, fmt.Errorf("corrupt state")
	}

	uc := NewExecUseCase(deps)
	err := uc.Execute(context.Background(), ExecOptions{
		Service: "api",
	})

	if err == nil {
		t.Fatal("expected error when state load fails, got nil")
	}
}

func TestExecUseCase_Execute_HostService(t *testing.T) {
	deps, configLoader, _, _, _, hostRunner := newTestDepsForExec(t)

	configLoader.LoadDepsFunc = func(configPath string) (*config.Deps, []string, error) {
		return &config.Deps{Project: config.Project{Name: "test-project"}}, nil, nil
	}
	hostRunner.LoadProcessesStateFunc = func(ws *workspace.Workspace) (map[string]*host.ProcessInfo, error) {
		return map[string]*host.ProcessInfo{
			"worker": {PID: 1234, Command: "npm start"},
		}, nil
	}

	uc := NewExecUseCase(deps)
	err := uc.Execute(context.Background(), ExecOptions{
		Service: "worker",
	})

	if err == nil {
		t.Fatal("expected error for host service, got nil")
	}
}

func TestExecUseCase_Execute_ServiceNotFound(t *testing.T) {
	deps, configLoader, _, _, dockerRunner, _ := newTestDepsForExec(t)

	configLoader.LoadDepsFunc = func(configPath string) (*config.Deps, []string, error) {
		return &config.Deps{Project: config.Project{Name: "test-project"}}, nil, nil
	}
	dockerRunner.GetAvailableServicesWithContextFunc = func(ctx context.Context, composePath string) ([]string, error) {
		return []string{"web", "db"}, nil
	}

	uc := NewExecUseCase(deps)
	err := uc.Execute(context.Background(), ExecOptions{
		Service: "api",
	})

	if err == nil {
		t.Fatal("expected error when service not found, got nil")
	}
}

func TestExecUseCase_Execute_GetServicesFails(t *testing.T) {
	deps, configLoader, _, _, dockerRunner, _ := newTestDepsForExec(t)

	configLoader.LoadDepsFunc = func(configPath string) (*config.Deps, []string, error) {
		return &config.Deps{Project: config.Project{Name: "test-project"}}, nil, nil
	}
	dockerRunner.GetAvailableServicesWithContextFunc = func(ctx context.Context, composePath string) ([]string, error) {
		return nil, fmt.Errorf("docker error")
	}

	uc := NewExecUseCase(deps)
	err := uc.Execute(context.Background(), ExecOptions{
		Service: "api",
	})

	if err == nil {
		t.Fatal("expected error when GetAvailableServices fails, got nil")
	}
}

func TestExecUseCase_Execute_DefaultCommand(t *testing.T) {
	deps, configLoader, _, _, dockerRunner, _ := newTestDepsForExec(t)

	configLoader.LoadDepsFunc = func(configPath string) (*config.Deps, []string, error) {
		return &config.Deps{Project: config.Project{Name: "test-project"}}, nil, nil
	}
	dockerRunner.GetAvailableServicesWithContextFunc = func(ctx context.Context, composePath string) ([]string, error) {
		return []string{"api", "web"}, nil
	}

	var capturedCommand []string
	dockerRunner.ExecInServiceFunc = func(ctx context.Context, composePath string, serviceName string, command []string, interactive bool) error {
		capturedCommand = command
		return nil
	}

	uc := NewExecUseCase(deps)
	err := uc.Execute(context.Background(), ExecOptions{
		Service: "api",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(capturedCommand) != 1 || capturedCommand[0] != "sh" {
		t.Errorf("expected default command [sh], got %v", capturedCommand)
	}
}

func TestExecUseCase_Execute_CustomCommand(t *testing.T) {
	deps, configLoader, _, _, dockerRunner, _ := newTestDepsForExec(t)

	configLoader.LoadDepsFunc = func(configPath string) (*config.Deps, []string, error) {
		return &config.Deps{Project: config.Project{Name: "test-project"}}, nil, nil
	}
	dockerRunner.GetAvailableServicesWithContextFunc = func(ctx context.Context, composePath string) ([]string, error) {
		return []string{"postgres"}, nil
	}

	var capturedService string
	var capturedCommand []string
	var capturedInteractive bool
	dockerRunner.ExecInServiceFunc = func(ctx context.Context, composePath string, serviceName string, command []string, interactive bool) error {
		capturedService = serviceName
		capturedCommand = command
		capturedInteractive = interactive
		return nil
	}

	uc := NewExecUseCase(deps)
	err := uc.Execute(context.Background(), ExecOptions{
		Service:     "postgres",
		Command:     []string{"psql", "-U", "postgres"},
		Interactive: true,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedService != "postgres" {
		t.Errorf("expected service 'postgres', got %q", capturedService)
	}
	if len(capturedCommand) != 3 || capturedCommand[0] != "psql" {
		t.Errorf("expected command [psql -U postgres], got %v", capturedCommand)
	}
	if !capturedInteractive {
		t.Error("expected interactive to be true")
	}
}

func TestExecUseCase_Execute_NonInteractive(t *testing.T) {
	deps, configLoader, _, _, dockerRunner, _ := newTestDepsForExec(t)

	configLoader.LoadDepsFunc = func(configPath string) (*config.Deps, []string, error) {
		return &config.Deps{Project: config.Project{Name: "test-project"}}, nil, nil
	}
	dockerRunner.GetAvailableServicesWithContextFunc = func(ctx context.Context, composePath string) ([]string, error) {
		return []string{"api"}, nil
	}

	var capturedInteractive bool
	dockerRunner.ExecInServiceFunc = func(ctx context.Context, composePath string, serviceName string, command []string, interactive bool) error {
		capturedInteractive = interactive
		return nil
	}

	uc := NewExecUseCase(deps)
	err := uc.Execute(context.Background(), ExecOptions{
		Service:     "api",
		Command:     []string{"ls", "-la"},
		Interactive: false,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedInteractive {
		t.Error("expected interactive to be false")
	}
}

func TestExecUseCase_Execute_WithProjectName(t *testing.T) {
	deps, configLoader, wsMgr, _, dockerRunner, _ := newTestDepsForExec(t)

	configLoader.LoadDepsFunc = func(configPath string) (*config.Deps, []string, error) {
		return &config.Deps{Project: config.Project{Name: "my-project"}}, nil, nil
	}

	var resolvedWorkspace string
	wsMgr.ResolveFunc = func(projectName string) (*workspace.Workspace, error) {
		resolvedWorkspace = projectName
		return &workspace.Workspace{Root: t.TempDir()}, nil
	}

	dockerRunner.GetAvailableServicesWithContextFunc = func(ctx context.Context, composePath string) ([]string, error) {
		return []string{"api"}, nil
	}
	dockerRunner.ExecInServiceFunc = func(ctx context.Context, composePath string, serviceName string, command []string, interactive bool) error {
		return nil
	}

	uc := NewExecUseCase(deps)
	err := uc.Execute(context.Background(), ExecOptions{
		ProjectName: "my-project",
		Service:     "api",
		Command:     []string{"sh"},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolvedWorkspace != "my-project" {
		t.Errorf("expected workspace 'my-project', got %q", resolvedWorkspace)
	}
}

func TestExecUseCase_Execute_ServiceInProjectCompose(t *testing.T) {
	deps, configLoader, _, stateMgr, dockerRunner, _ := newTestDepsForExec(t)

	configLoader.LoadDepsFunc = func(configPath string) (*config.Deps, []string, error) {
		return &config.Deps{Project: config.Project{Name: "test-project"}}, nil, nil
	}

	projectComposePath := "/path/to/project/docker-compose.yml"
	stateMgr.LoadFunc = func(ws *workspace.Workspace) (*config.Deps, error) {
		return &config.Deps{
			Project:            config.Project{Name: "test-project"},
			ProjectComposePath: projectComposePath,
		}, nil
	}

	// Service NOT in generated compose
	callCount := 0
	dockerRunner.GetAvailableServicesWithContextFunc = func(ctx context.Context, composePath string) ([]string, error) {
		callCount++
		if callCount == 1 {
			// Generated compose - service not here
			return []string{"api", "db"}, nil
		}
		// Project compose - service is here
		return []string{"frontend", "nginx"}, nil
	}

	var capturedComposePath string
	dockerRunner.ExecInServiceFunc = func(ctx context.Context, composePath string, serviceName string, command []string, interactive bool) error {
		capturedComposePath = composePath
		return nil
	}

	uc := NewExecUseCase(deps)
	err := uc.Execute(context.Background(), ExecOptions{
		Service: "frontend",
		Command: []string{"sh"},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedComposePath != projectComposePath {
		t.Errorf("expected compose path %q, got %q", projectComposePath, capturedComposePath)
	}
}

func TestExecUseCase_Execute_NilContext(t *testing.T) {
	deps, configLoader, _, _, dockerRunner, _ := newTestDepsForExec(t)

	configLoader.LoadDepsFunc = func(configPath string) (*config.Deps, []string, error) {
		return &config.Deps{Project: config.Project{Name: "test-project"}}, nil, nil
	}
	dockerRunner.GetAvailableServicesWithContextFunc = func(ctx context.Context, composePath string) ([]string, error) {
		return []string{"api"}, nil
	}
	dockerRunner.ExecInServiceFunc = func(ctx context.Context, composePath string, serviceName string, command []string, interactive bool) error {
		if ctx == nil {
			t.Error("expected non-nil context")
		}
		return nil
	}

	uc := NewExecUseCase(deps)
	//nolint:staticcheck // testing nil context handling
	err := uc.Execute(nil, ExecOptions{
		Service: "api",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
