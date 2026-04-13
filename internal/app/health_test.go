package app

import (
	"context"
	"fmt"
	"testing"

	"raioz/internal/config"
	"raioz/internal/mocks"
)

func TestNewHealthUseCase(t *testing.T) {
	deps := newFullMockDeps()
	uc := NewHealthUseCase(deps)
	if uc == nil {
		t.Fatal("expected non-nil HealthUseCase")
	}
}

func TestHealthUseCase_Execute_ConfigLoadError(t *testing.T) {
	initI18nForTest(t)
	deps := newFullMockDeps()
	deps.ConfigLoader = &mocks.MockConfigLoader{
		LoadDepsFunc: func(configPath string) (*config.Deps, []string, error) {
			return nil, nil, fmt.Errorf("fail")
		},
	}
	uc := NewHealthUseCase(deps)
	err := uc.Execute(context.Background(), HealthOptions{ConfigPath: "bad.json"})
	if err == nil {
		t.Fatal("expected error for config load failure, got nil")
	}
}

func TestHealthUseCase_Execute_BaseDirError(t *testing.T) {
	initI18nForTest(t)
	deps := newFullMockDeps()
	deps.ConfigLoader = &mocks.MockConfigLoader{
		LoadDepsFunc: func(configPath string) (*config.Deps, []string, error) {
			return &config.Deps{Project: config.Project{Name: "test"}}, nil, nil
		},
	}
	deps.Workspace = &mocks.MockWorkspaceManager{
		GetBaseDirFunc: func() (string, error) {
			return "", fmt.Errorf("base dir error")
		},
	}
	uc := NewHealthUseCase(deps)
	err := uc.Execute(context.Background(), HealthOptions{ConfigPath: "ok.json"})
	if err == nil {
		t.Fatal("expected error for base dir failure, got nil")
	}
}

func TestHealthUseCase_Execute_NotLocal(t *testing.T) {
	initI18nForTest(t)
	baseDir := t.TempDir()
	deps := newFullMockDeps()
	deps.ConfigLoader = &mocks.MockConfigLoader{
		LoadDepsFunc: func(configPath string) (*config.Deps, []string, error) {
			return &config.Deps{Project: config.Project{Name: "test"}}, nil, nil
		},
	}
	deps.Workspace = &mocks.MockWorkspaceManager{
		GetBaseDirFunc: func() (string, error) { return baseDir, nil },
	}
	uc := NewHealthUseCase(deps)
	// Config path inside baseDir/workspaces -> not local
	configPath := baseDir + "/workspaces/myproj/raioz.json"
	err := uc.Execute(context.Background(), HealthOptions{ConfigPath: configPath})
	if err != nil {
		t.Fatalf("unexpected error for non-local project: %v", err)
	}
}
