package app

import (
	"context"
	"fmt"
	"testing"

	"raioz/internal/config"
	"raioz/internal/domain/interfaces"
	"raioz/internal/workspace"
)

func TestLogsUseCase_Execute_WorkspaceResolveFails(t *testing.T) {
	initI18nForTest(t)
	deps, configLoader, wsMgr, _, _ := newTestDepsForLogs(t)
	configLoader.LoadDepsFunc = func(configPath string) (*config.Deps, []string, error) {
		return &config.Deps{Project: config.Project{Name: "test-project"}}, nil, nil
	}
	wsMgr.ResolveFunc = func(name string) (*workspace.Workspace, error) {
		return nil, fmt.Errorf("no workspace")
	}
	uc := NewLogsUseCase(deps)
	err := uc.Execute(context.Background(), LogsOptions{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestLogsUseCase_Execute_StateLoadFails(t *testing.T) {
	initI18nForTest(t)
	deps, configLoader, _, stateMgr, _ := newTestDepsForLogs(t)
	configLoader.LoadDepsFunc = func(configPath string) (*config.Deps, []string, error) {
		return &config.Deps{Project: config.Project{Name: "test-project"}}, nil, nil
	}
	stateMgr.LoadFunc = func(ws *workspace.Workspace) (*config.Deps, error) {
		return nil, fmt.Errorf("corrupt")
	}
	uc := NewLogsUseCase(deps)
	err := uc.Execute(context.Background(), LogsOptions{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestLogsUseCase_Execute_SpecificServices(t *testing.T) {
	initI18nForTest(t)
	deps, configLoader, _, _, dockerRunner := newTestDepsForLogs(t)
	configLoader.LoadDepsFunc = func(configPath string) (*config.Deps, []string, error) {
		return &config.Deps{Project: config.Project{Name: "test-project"}}, nil, nil
	}
	dockerRunner.GetAvailableServicesWithContextFunc = func(ctx context.Context, composePath string) ([]string, error) {
		return []string{"api", "web"}, nil
	}
	var captured []string
	dockerRunner.ViewLogsWithContextFunc = func(ctx context.Context, composePath string, opts interfaces.LogsOptions) error {
		captured = opts.Services
		return nil
	}
	uc := NewLogsUseCase(deps)
	err := uc.Execute(context.Background(), LogsOptions{
		Services: []string{"api"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(captured) != 1 || captured[0] != "api" {
		t.Errorf("expected [api], got %v", captured)
	}
}

func TestLogsUseCase_Execute_WithProjectName(t *testing.T) {
	initI18nForTest(t)
	deps, configLoader, _, _, dockerRunner := newTestDepsForLogs(t)
	configLoader.LoadDepsFunc = func(configPath string) (*config.Deps, []string, error) {
		return &config.Deps{Project: config.Project{Name: "my-proj"}}, nil, nil
	}
	dockerRunner.GetAvailableServicesWithContextFunc = func(ctx context.Context, composePath string) ([]string, error) {
		return []string{"api"}, nil
	}
	dockerRunner.ViewLogsWithContextFunc = func(ctx context.Context, composePath string, opts interfaces.LogsOptions) error {
		return nil
	}
	uc := NewLogsUseCase(deps)
	err := uc.Execute(context.Background(), LogsOptions{
		ProjectName: "my-proj",
		All:         true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
