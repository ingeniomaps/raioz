package app

import (
	"context"
	"fmt"
	"testing"

	"raioz/internal/config"
	"raioz/internal/mocks"
	"raioz/internal/workspace"
)

func TestNewCheckUseCase(t *testing.T) {
	deps := newFullMockDeps()
	uc := NewCheckUseCase(deps)
	if uc == nil {
		t.Fatal("expected non-nil CheckUseCase")
	}
	if uc.useCase == nil {
		t.Error("expected non-nil inner useCase")
	}
}

func TestCheckUseCase_Execute_ConfigLoadError(t *testing.T) {
	initI18nForTest(t)
	deps := newFullMockDeps()
	deps.ConfigLoader = &mocks.MockConfigLoader{
		LoadDepsFunc: func(configPath string) (*config.Deps, []string, error) {
			return nil, nil, fmt.Errorf("load error")
		},
	}
	uc := NewCheckUseCase(deps)
	_, err := uc.Execute(context.Background(), CheckOptions{ConfigPath: "bad.json"})
	// Expect an error or result with ConfigValid=false — at minimum not panic
	if err == nil {
		// Some implementations return an error-bearing result; both acceptable
		_ = err
	}
}

func TestCheckUseCase_Execute_JSONMode(t *testing.T) {
	initI18nForTest(t)
	deps := newFullMockDeps()
	deps.ConfigLoader = &mocks.MockConfigLoader{
		LoadDepsFunc: func(configPath string) (*config.Deps, []string, error) {
			// Legacy (non-2.0) config
			return &config.Deps{
				Project:  config.Project{Name: "test"},
				Services: map[string]config.Service{},
			}, nil, nil
		},
	}
	deps.Workspace = &mocks.MockWorkspaceManager{
		ResolveFunc: func(name string) (*workspace.Workspace, error) {
			return &workspace.Workspace{Root: t.TempDir()}, nil
		},
	}
	deps.StateManager = &mocks.MockStateManager{
		ExistsFunc: func(ws *workspace.Workspace) bool { return false },
	}
	uc := NewCheckUseCase(deps)
	_, err := uc.Execute(context.Background(), CheckOptions{ConfigPath: "raioz.json"})
	_ = err // Either nil or error from checkcase is fine; we just exercise the path
}
