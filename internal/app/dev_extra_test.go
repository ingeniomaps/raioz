package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"raioz/internal/config"
	"raioz/internal/mocks"
	"raioz/internal/state"
)

// TestDevUseCase_Execute_PromoteMissingPath tries to promote but path is not found.
func TestDevUseCase_Execute_PromoteMissingPath(t *testing.T) {
	initI18nForTest(t)
	tmpDir := t.TempDir()
	deps := newFullMockDeps()
	deps.ConfigLoader = &mocks.MockConfigLoader{
		LoadDepsFunc: func(p string) (*config.Deps, []string, error) {
			return &config.Deps{
				Project: config.Project{Name: "proj"},
				Infra: map[string]config.InfraEntry{
					"redis": {Inline: &config.Infra{Image: "redis", Tag: "7"}},
				},
			}, nil, nil
		},
	}
	uc := NewDevUseCase(deps)
	err := uc.Execute(context.Background(), DevOptions{
		ConfigPath: filepath.Join(tmpDir, "raioz.yaml"),
		Name:       "redis",
		LocalPath:  "/nonexistent/path/xxx",
	})
	if err == nil {
		t.Error("expected error for missing local path")
	}
}

// TestDevUseCase_Execute_PromoteNotADependency: non-existent infra name.
func TestDevUseCase_Execute_PromoteNotADependency(t *testing.T) {
	initI18nForTest(t)
	tmpDir := t.TempDir()
	deps := newFullMockDeps()
	deps.ConfigLoader = &mocks.MockConfigLoader{
		LoadDepsFunc: func(p string) (*config.Deps, []string, error) {
			return &config.Deps{
				Project: config.Project{Name: "proj"},
				Infra:   map[string]config.InfraEntry{},
			}, nil, nil
		},
	}
	uc := NewDevUseCase(deps)
	err := uc.Execute(context.Background(), DevOptions{
		ConfigPath: filepath.Join(tmpDir, "raioz.yaml"),
		Name:       "unknown",
		LocalPath:  tmpDir,
	})
	if err == nil {
		t.Error("expected error for unknown dependency")
	}
}

// TestDevUseCase_Execute_PromoteNoLocalPath
func TestDevUseCase_Execute_PromoteNoLocalPath(t *testing.T) {
	initI18nForTest(t)
	tmpDir := t.TempDir()
	deps := newFullMockDeps()
	deps.ConfigLoader = &mocks.MockConfigLoader{
		LoadDepsFunc: func(p string) (*config.Deps, []string, error) {
			return &config.Deps{
				Project: config.Project{Name: "proj"},
				Infra: map[string]config.InfraEntry{
					"redis": {Inline: &config.Infra{Image: "redis"}},
				},
			}, nil, nil
		},
	}
	uc := NewDevUseCase(deps)
	err := uc.Execute(context.Background(), DevOptions{
		ConfigPath: filepath.Join(tmpDir, "raioz.yaml"),
		Name:       "redis",
		LocalPath:  "",
	})
	if err == nil {
		t.Error("expected error when local path empty")
	}
}

// TestDevUseCase_Execute_PromoteRuntimeUnknown
func TestDevUseCase_Execute_PromoteRuntimeUnknown(t *testing.T) {
	initI18nForTest(t)
	tmpDir := t.TempDir()
	// Create empty directory (no runtime detection)
	empty := filepath.Join(tmpDir, "empty")
	if err := os.Mkdir(empty, 0755); err != nil {
		t.Fatal(err)
	}
	deps := newFullMockDeps()
	deps.ConfigLoader = &mocks.MockConfigLoader{
		LoadDepsFunc: func(p string) (*config.Deps, []string, error) {
			return &config.Deps{
				Project: config.Project{Name: "proj"},
				Infra: map[string]config.InfraEntry{
					"redis": {Inline: &config.Infra{Image: "redis"}},
				},
			}, nil, nil
		},
	}
	uc := NewDevUseCase(deps)
	err := uc.Execute(context.Background(), DevOptions{
		ConfigPath: filepath.Join(tmpDir, "raioz.yaml"),
		Name:       "redis",
		LocalPath:  empty,
	})
	if err == nil {
		t.Error("expected error for unknown runtime")
	}
}

// TestDevUseCase_Execute_ResetNotInDevMode
func TestDevUseCase_Execute_ResetNotInDevMode(t *testing.T) {
	initI18nForTest(t)
	tmpDir := t.TempDir()
	deps := newFullMockDeps()
	deps.ConfigLoader = &mocks.MockConfigLoader{
		LoadDepsFunc: func(p string) (*config.Deps, []string, error) {
			return &config.Deps{
				Project: config.Project{Name: "proj"},
				Infra: map[string]config.InfraEntry{
					"redis": {Inline: &config.Infra{Image: "redis"}},
				},
			}, nil, nil
		},
	}
	uc := NewDevUseCase(deps)
	err := uc.Execute(context.Background(), DevOptions{
		ConfigPath: filepath.Join(tmpDir, "raioz.yaml"),
		Name:       "redis",
		Reset:      true,
	})
	if err == nil {
		t.Error("expected error when not in dev mode")
	}
}

// TestDevUseCase_Execute_ListWithState ensures listOverrides is called with state
func TestDevUseCase_Execute_ListWithState(t *testing.T) {
	initI18nForTest(t)
	tmpDir := t.TempDir()
	// Seed a state file with a dev override
	ls := &state.LocalState{
		Project: "proj",
		DevOverrides: map[string]state.DevOverride{
			"pg": {OriginalImage: "postgres:16", LocalPath: "/tmp/pg"},
		},
	}
	if err := state.SaveLocalState(tmpDir, ls); err != nil {
		t.Fatal(err)
	}

	deps := newFullMockDeps()
	uc := NewDevUseCase(deps)
	err := uc.Execute(context.Background(), DevOptions{
		ConfigPath: filepath.Join(tmpDir, "raioz.yaml"),
		List:       true,
	})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}
