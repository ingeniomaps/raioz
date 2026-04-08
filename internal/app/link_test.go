package app

import (
	"os"
	"path/filepath"
	"testing"

	"raioz/internal/config"
	"raioz/internal/i18n"
	"raioz/internal/mocks"
	"raioz/internal/workspace"
)

func initI18nLink(t *testing.T) {
	t.Helper()
	os.Setenv("RAIOZ_LANG", "en")
	t.Cleanup(func() { os.Unsetenv("RAIOZ_LANG") })
	i18n.Init("en")
}

func TestNewLinkUseCase(t *testing.T) {
	uc := NewLinkUseCase(&Dependencies{})
	if uc == nil {
		t.Fatal("should return non-nil")
	}
}

func TestLinkAddConfigLoadError(t *testing.T) {
	initI18nLink(t)

	uc := NewLinkUseCase(&Dependencies{
		ConfigLoader: &mocks.MockConfigLoader{
			LoadDepsFunc: func(path string) (*config.Deps, []string, error) {
				return nil, nil, os.ErrNotExist
			},
		},
	})

	err := uc.Add("svc", "/tmp", "bad.json")
	if err == nil {
		t.Error("expected error when config load fails")
	}
}

func TestLinkAddServiceNotFound(t *testing.T) {
	initI18nLink(t)

	ws := &workspace.Workspace{Root: "/tmp/test"}
	cfgDeps := &config.Deps{
		Project:  config.Project{Name: "test"},
		Services: map[string]config.Service{},
	}

	uc := NewLinkUseCase(&Dependencies{
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
	})

	err := uc.Add("ghost", "/tmp", "config.json")
	if err == nil {
		t.Error("expected error for service not in config")
	}
}

func TestLinkAddSuccess(t *testing.T) {
	initI18nLink(t)

	tmpDir := t.TempDir()
	extDir := t.TempDir()
	svcDir := filepath.Join(tmpDir, "services", "api")

	ws := &workspace.Workspace{Root: tmpDir, ServicesDir: filepath.Join(tmpDir, "services")}
	cfgDeps := &config.Deps{
		Project: config.Project{Name: "test"},
		Services: map[string]config.Service{
			"api": {Source: config.SourceConfig{Kind: "image", Image: "nginx"}},
		},
	}

	uc := NewLinkUseCase(&Dependencies{
		ConfigLoader: &mocks.MockConfigLoader{
			LoadDepsFunc: func(path string) (*config.Deps, []string, error) {
				return cfgDeps, nil, nil
			},
		},
		Workspace: &mocks.MockWorkspaceManager{
			ResolveFunc: func(name string) (*workspace.Workspace, error) {
				return ws, nil
			},
			GetServicePathFunc: func(ws *workspace.Workspace, serviceName string, svc config.Service) string {
				return svcDir
			},
		},
	})

	err := uc.Add("api", extDir, "config.json")
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	// Verify symlink was created
	info, err := os.Lstat(svcDir)
	if err != nil {
		t.Fatalf("symlink not created: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("expected symlink, got regular file/dir")
	}
}

func TestLinkRemoveNotLinked(t *testing.T) {
	initI18nLink(t)

	tmpDir := t.TempDir()
	svcDir := filepath.Join(tmpDir, "services", "api")
	os.MkdirAll(svcDir, 0755)

	ws := &workspace.Workspace{Root: tmpDir}
	cfgDeps := &config.Deps{
		Project: config.Project{Name: "test"},
		Services: map[string]config.Service{
			"api": {Source: config.SourceConfig{Kind: "image"}},
		},
	}

	uc := NewLinkUseCase(&Dependencies{
		ConfigLoader: &mocks.MockConfigLoader{
			LoadDepsFunc: func(path string) (*config.Deps, []string, error) {
				return cfgDeps, nil, nil
			},
		},
		Workspace: &mocks.MockWorkspaceManager{
			ResolveFunc: func(name string) (*workspace.Workspace, error) {
				return ws, nil
			},
			GetServicePathFunc: func(ws *workspace.Workspace, serviceName string, svc config.Service) string {
				return svcDir
			},
		},
	})

	// Should not error — prints info message
	err := uc.Remove("api", "config.json")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
}

func TestLinkListEmpty(t *testing.T) {
	initI18nLink(t)

	tmpDir := t.TempDir()
	ws := &workspace.Workspace{Root: tmpDir}
	cfgDeps := &config.Deps{
		Project:  config.Project{Name: "test"},
		Services: map[string]config.Service{},
	}

	uc := NewLinkUseCase(&Dependencies{
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
	})

	err := uc.List("config.json")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
}
