package upcase

import (
	"context"
	"os"
	"testing"

	"raioz/internal/domain/models"
	"raioz/internal/i18n"
	"raioz/internal/mocks"
	"raioz/internal/workspace"
)

func initI18nUp(t *testing.T) {
	t.Helper()
	os.Setenv("RAIOZ_LANG", "en")
	t.Cleanup(func() { os.Unsetenv("RAIOZ_LANG") })
	i18n.Init("en")
}

func validDeps() *models.Deps {
	return &models.Deps{
		SchemaVersion: "1.0",
		Network:       models.NetworkConfig{Name: "test-net"},
		Project:       models.Project{Name: "test-project"},
		Services: map[string]models.Service{
			"api": {
				Source: models.SourceConfig{Kind: "image", Image: "nginx", Tag: "latest"},
				Docker: &models.DockerConfig{Mode: "prod", Ports: []string{"3000:3000"}},
			},
		},
		Infra: map[string]models.InfraEntry{},
		Env:   models.EnvConfig{UseGlobal: true, Files: []string{"global"}},
	}
}

func TestBootstrapSuccess(t *testing.T) {
	initI18nUp(t)

	deps := validDeps()
	ws := &workspace.Workspace{Root: "/tmp/test", ServicesDir: "/tmp/test/services"}

	uc := NewUseCase(&Dependencies{
		ConfigLoader: &mocks.MockConfigLoader{
			LoadDepsFunc: func(path string) (*models.Deps, []string, error) {
				return deps, nil, nil
			},
		},
		Workspace: &mocks.MockWorkspaceManager{
			ResolveFunc: func(name string) (*workspace.Workspace, error) {
				return ws, nil
			},
		},
	})

	result, err := uc.bootstrap(context.Background(), ".raioz.json")
	if err != nil {
		t.Fatalf("bootstrap() error: %v", err)
	}

	if result.deps.Project.Name != "test-project" {
		t.Errorf("deps.Project.Name = %s, want test-project", result.deps.Project.Name)
	}
	if result.ws == nil {
		t.Error("workspace should not be nil")
	}
	if result.ctx == nil {
		t.Error("context should not be nil")
	}
}

func TestBootstrapConfigLoadError(t *testing.T) {
	initI18nUp(t)

	uc := NewUseCase(&Dependencies{
		ConfigLoader: &mocks.MockConfigLoader{
			LoadDepsFunc: func(path string) (*models.Deps, []string, error) {
				return nil, nil, os.ErrNotExist
			},
		},
	})

	_, err := uc.bootstrap(context.Background(), "nonexistent.json")
	if err == nil {
		t.Error("expected error when config loading fails")
	}
}

func TestBootstrapWorkspaceResolveError(t *testing.T) {
	initI18nUp(t)

	deps := validDeps()

	uc := NewUseCase(&Dependencies{
		ConfigLoader: &mocks.MockConfigLoader{
			LoadDepsFunc: func(path string) (*models.Deps, []string, error) {
				return deps, nil, nil
			},
		},
		Workspace: &mocks.MockWorkspaceManager{
			ResolveFunc: func(name string) (*workspace.Workspace, error) {
				return nil, os.ErrPermission
			},
		},
	})

	_, err := uc.bootstrap(context.Background(), ".raioz.json")
	if err == nil {
		t.Error("expected error when workspace resolve fails")
	}
}

func TestBootstrapShowsWarnings(t *testing.T) {
	initI18nUp(t)

	deps := validDeps()
	ws := &workspace.Workspace{Root: "/tmp/test"}

	uc := NewUseCase(&Dependencies{
		ConfigLoader: &mocks.MockConfigLoader{
			LoadDepsFunc: func(path string) (*models.Deps, []string, error) {
				return deps, []string{"deprecation warning"}, nil
			},
		},
		Workspace: &mocks.MockWorkspaceManager{
			ResolveFunc: func(name string) (*workspace.Workspace, error) {
				return ws, nil
			},
		},
	})

	result, err := uc.bootstrap(context.Background(), ".raioz.json")
	if err != nil {
		t.Fatalf("bootstrap() error: %v", err)
	}
	if result.deps == nil {
		t.Error("deps should not be nil even with warnings")
	}
}

func TestBootstrapNilContext(t *testing.T) {
	initI18nUp(t)

	deps := validDeps()
	ws := &workspace.Workspace{Root: "/tmp/test"}

	uc := NewUseCase(&Dependencies{
		ConfigLoader: &mocks.MockConfigLoader{
			LoadDepsFunc: func(path string) (*models.Deps, []string, error) {
				return deps, nil, nil
			},
		},
		Workspace: &mocks.MockWorkspaceManager{
			ResolveFunc: func(name string) (*workspace.Workspace, error) {
				return ws, nil
			},
		},
	})

	//nolint:staticcheck // intentionally passing nil context for test
	result, err := uc.bootstrap(nil, ".raioz.json")
	if err != nil {
		t.Fatalf("bootstrap() error: %v", err)
	}
	if result.ctx == nil {
		t.Error("should create context when nil is passed")
	}
}
