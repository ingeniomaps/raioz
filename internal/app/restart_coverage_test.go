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

func TestRestartUseCase_Execute_HostServiceBlocked(t *testing.T) {
	initI18nForTest(t)
	deps, configLoader, _, _, _, hostRunner := newTestDepsForRestart(t)
	configLoader.LoadDepsFunc = func(configPath string) (*config.Deps, []string, error) {
		return &config.Deps{Project: config.Project{Name: "proj"}}, nil, nil
	}
	hostRunner.LoadProcessesStateFunc = func(ws *workspace.Workspace) (map[string]*host.ProcessInfo, error) {
		return map[string]*host.ProcessInfo{
			"api": {PID: 1234},
		}, nil
	}
	uc := NewRestartUseCase(deps)
	err := uc.Execute(context.Background(), RestartOptions{
		Services: []string{"api"},
	})
	if err == nil {
		t.Fatal("expected error for host service restart attempt")
	}
}

func TestRestartUseCase_Execute_AllExcludeInfra(t *testing.T) {
	initI18nForTest(t)
	deps, configLoader, _, stateMgr, dockerRunner, _ := newTestDepsForRestart(t)
	configLoader.LoadDepsFunc = func(configPath string) (*config.Deps, []string, error) {
		return &config.Deps{Project: config.Project{Name: "proj"}}, nil, nil
	}
	stateMgr.LoadFunc = func(ws *workspace.Workspace) (*config.Deps, error) {
		return &config.Deps{
			Project: config.Project{Name: "proj"},
			Infra: map[string]config.InfraEntry{
				"redis": {Inline: &config.Infra{Image: "redis:7"}},
			},
		}, nil
	}
	dockerRunner.GetAvailableServicesWithContextFunc = func(ctx context.Context, composePath string) ([]string, error) {
		return []string{"api", "redis"}, nil
	}
	var capturedServices []string
	dockerRunner.RestartServicesWithContextFunc = func(ctx context.Context, composePath string, serviceNames []string) error {
		capturedServices = serviceNames
		return nil
	}
	uc := NewRestartUseCase(deps)
	err := uc.Execute(context.Background(), RestartOptions{All: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should exclude redis (infra)
	for _, svc := range capturedServices {
		if svc == "redis" {
			t.Error("expected redis to be excluded from restart (infra)")
		}
	}
}

func TestRestartUseCase_Execute_RestartFails(t *testing.T) {
	initI18nForTest(t)
	deps, configLoader, _, _, dockerRunner, _ := newTestDepsForRestart(t)
	configLoader.LoadDepsFunc = func(configPath string) (*config.Deps, []string, error) {
		return &config.Deps{Project: config.Project{Name: "proj"}}, nil, nil
	}
	dockerRunner.GetAvailableServicesWithContextFunc = func(ctx context.Context, composePath string) ([]string, error) {
		return []string{"api"}, nil
	}
	dockerRunner.RestartServicesWithContextFunc = func(ctx context.Context, composePath string, serviceNames []string) error {
		return fmt.Errorf("restart fail")
	}
	uc := NewRestartUseCase(deps)
	err := uc.Execute(context.Background(), RestartOptions{All: true, IncludeInfra: true})
	if err == nil {
		t.Fatal("expected error for restart failure")
	}
}

func TestRestartUseCase_Execute_ProjectNameProvided(t *testing.T) {
	initI18nForTest(t)
	deps, configLoader, _, _, dockerRunner, _ := newTestDepsForRestart(t)
	configLoader.LoadDepsFunc = func(configPath string) (*config.Deps, []string, error) {
		return &config.Deps{Project: config.Project{Name: "proj"}}, nil, nil
	}
	dockerRunner.GetAvailableServicesWithContextFunc = func(ctx context.Context, composePath string) ([]string, error) {
		return []string{"api"}, nil
	}
	dockerRunner.RestartServicesWithContextFunc = func(ctx context.Context, composePath string, serviceNames []string) error {
		return nil
	}
	uc := NewRestartUseCase(deps)
	err := uc.Execute(context.Background(), RestartOptions{
		ProjectName: "proj",
		All:         true,
		IncludeInfra: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRestartUseCase_Execute_ProjectNameMismatch(t *testing.T) {
	initI18nForTest(t)
	deps, configLoader, _, _, dockerRunner, _ := newTestDepsForRestart(t)
	configLoader.LoadDepsFunc = func(configPath string) (*config.Deps, []string, error) {
		return &config.Deps{Project: config.Project{Name: "other-proj"}}, nil, nil
	}
	dockerRunner.GetAvailableServicesWithContextFunc = func(ctx context.Context, composePath string) ([]string, error) {
		return []string{"api"}, nil
	}
	dockerRunner.RestartServicesWithContextFunc = func(ctx context.Context, composePath string, serviceNames []string) error {
		return nil
	}
	uc := NewRestartUseCase(deps)
	err := uc.Execute(context.Background(), RestartOptions{
		ProjectName:  "my-proj", // doesn't match config's "other-proj"
		All:          true,
		IncludeInfra: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRestartUseCase_resolveProjectComposeServices_Available(t *testing.T) {
	initI18nForTest(t)
	deps := newFullMockDeps()
	deps.DockerRunner = &mocks.MockDockerRunner{
		GetAvailableServicesWithContextFunc: func(ctx context.Context, composePath string) ([]string, error) {
			return []string{"db", "web"}, nil
		},
	}
	uc := NewRestartUseCase(deps)
	found, err := uc.resolveProjectComposeServices(context.Background(), RestartOptions{
		Services: []string{"db"},
	}, "/tmp/compose.yml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(found) != 1 || found[0] != "db" {
		t.Errorf("expected [db], got %v", found)
	}
}

func TestRestartUseCase_resolveProjectComposeServices_NotFound(t *testing.T) {
	initI18nForTest(t)
	deps := newFullMockDeps()
	deps.DockerRunner = &mocks.MockDockerRunner{
		GetAvailableServicesWithContextFunc: func(ctx context.Context, composePath string) ([]string, error) {
			return []string{"db"}, nil
		},
	}
	uc := NewRestartUseCase(deps)
	_, err := uc.resolveProjectComposeServices(context.Background(), RestartOptions{
		Services: []string{"missing"},
	}, "/tmp/compose.yml")
	if err == nil {
		t.Fatal("expected error for service not found")
	}
}

func TestRestartUseCase_resolveProjectComposeServices_Error(t *testing.T) {
	initI18nForTest(t)
	deps := newFullMockDeps()
	deps.DockerRunner = &mocks.MockDockerRunner{
		GetAvailableServicesWithContextFunc: func(ctx context.Context, composePath string) ([]string, error) {
			return nil, fmt.Errorf("compose error")
		},
	}
	uc := NewRestartUseCase(deps)
	found, err := uc.resolveProjectComposeServices(context.Background(), RestartOptions{
		Services: []string{"api"},
	}, "/tmp/compose.yml")
	if err != nil {
		t.Fatal("expected nil error for compose failure (graceful)")
	}
	if found != nil {
		t.Errorf("expected nil, got %v", found)
	}
}

func TestRestartUseCase_Execute_FallbackToProjectCompose(t *testing.T) {
	initI18nForTest(t)
	deps, configLoader, _, stateMgr, dockerRunner, _ := newTestDepsForRestart(t)
	configLoader.LoadDepsFunc = func(configPath string) (*config.Deps, []string, error) {
		return &config.Deps{Project: config.Project{Name: "proj"}}, nil, nil
	}
	stateMgr.LoadFunc = func(ws *workspace.Workspace) (*config.Deps, error) {
		return &config.Deps{
			Project:            config.Project{Name: "proj"},
			ProjectComposePath: "/tmp/project-compose.yml",
		}, nil
	}
	callCount := 0
	dockerRunner.GetAvailableServicesWithContextFunc = func(ctx context.Context, composePath string) ([]string, error) {
		callCount++
		if callCount == 1 {
			// First call: generated compose - service not found
			return []string{}, nil
		}
		// Second call: project compose - service found
		return []string{"custom-svc"}, nil
	}
	dockerRunner.RestartServicesWithContextFunc = func(ctx context.Context, composePath string, serviceNames []string) error {
		return nil
	}
	uc := NewRestartUseCase(deps)
	err := uc.Execute(context.Background(), RestartOptions{
		Services: []string{"custom-svc"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
