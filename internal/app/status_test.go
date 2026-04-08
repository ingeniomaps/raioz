package app

import (
	"context"
	"os"
	"testing"

	"raioz/internal/config"
	"raioz/internal/i18n"
	"raioz/internal/mocks"
	"raioz/internal/workspace"
)

func initI18nStatus(t *testing.T) {
	t.Helper()
	os.Setenv("RAIOZ_LANG", "en")
	t.Cleanup(func() { os.Unsetenv("RAIOZ_LANG") })
	i18n.Init("en")
}

func TestNewStatusUseCase(t *testing.T) {
	uc := NewStatusUseCase(&Dependencies{})
	if uc == nil {
		t.Fatal("should return non-nil")
	}
}

func TestStatusNoConfig(t *testing.T) {
	initI18nStatus(t)

	uc := NewStatusUseCase(&Dependencies{
		ConfigLoader: &mocks.MockConfigLoader{
			LoadDepsFunc: func(path string) (*config.Deps, []string, error) {
				return nil, nil, nil
			},
		},
	})

	err := uc.Execute(context.Background(), StatusOptions{ConfigPath: "bad.json"})
	if err == nil {
		t.Error("expected error when config cannot determine project")
	}
}

func TestStatusNoState(t *testing.T) {
	initI18nStatus(t)

	cfgDeps := &config.Deps{
		Project:  config.Project{Name: "test"},
		Services: map[string]config.Service{},
	}
	ws := &workspace.Workspace{Root: "/tmp/test"}

	uc := NewStatusUseCase(&Dependencies{
		ConfigLoader: &mocks.MockConfigLoader{
			LoadDepsFunc: func(path string) (*config.Deps, []string, error) {
				return cfgDeps, nil, nil
			},
		},
		Workspace: &mocks.MockWorkspaceManager{
			ResolveFunc: func(name string) (*workspace.Workspace, error) {
				return ws, nil
			},
		},
		StateManager: &mocks.MockStateManager{
			ExistsFunc: func(ws *workspace.Workspace) bool {
				return false
			},
		},
	})

	err := uc.Execute(context.Background(), StatusOptions{ConfigPath: "config.json"})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
}

func TestStatusNoStateJSON(t *testing.T) {
	initI18nStatus(t)

	cfgDeps := &config.Deps{
		Project:  config.Project{Name: "test"},
		Services: map[string]config.Service{},
	}
	ws := &workspace.Workspace{Root: "/tmp/test"}

	uc := NewStatusUseCase(&Dependencies{
		ConfigLoader: &mocks.MockConfigLoader{
			LoadDepsFunc: func(path string) (*config.Deps, []string, error) {
				return cfgDeps, nil, nil
			},
		},
		Workspace: &mocks.MockWorkspaceManager{
			ResolveFunc: func(name string) (*workspace.Workspace, error) {
				return ws, nil
			},
		},
		StateManager: &mocks.MockStateManager{
			ExistsFunc: func(ws *workspace.Workspace) bool {
				return false
			},
		},
	})

	err := uc.Execute(context.Background(), StatusOptions{ConfigPath: "config.json", JSON: true})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
}

func TestStatusWithProjectName(t *testing.T) {
	initI18nStatus(t)

	ws := &workspace.Workspace{Root: "/tmp/test"}

	uc := NewStatusUseCase(&Dependencies{
		ConfigLoader: &mocks.MockConfigLoader{
			LoadDepsFunc: func(path string) (*config.Deps, []string, error) {
				return nil, nil, nil
			},
		},
		Workspace: &mocks.MockWorkspaceManager{
			ResolveFunc: func(name string) (*workspace.Workspace, error) {
				return ws, nil
			},
		},
		StateManager: &mocks.MockStateManager{
			ExistsFunc: func(ws *workspace.Workspace) bool {
				return false
			},
		},
	})

	err := uc.Execute(context.Background(), StatusOptions{ProjectName: "my-proj", ConfigPath: "config.json"})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
}
