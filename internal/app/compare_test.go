package app

import (
	"fmt"
	"path/filepath"
	"testing"

	"raioz/internal/config"
	"raioz/internal/mocks"
)

func TestNewCompareUseCase(t *testing.T) {
	deps := newFullMockDeps()
	uc := NewCompareUseCase(deps)
	if uc == nil {
		t.Fatal("expected non-nil CompareUseCase")
	}
}

func TestCompareUseCase_Execute_MissingProductionPath(t *testing.T) {
	initI18nForTest(t)
	deps := newFullMockDeps()
	uc := NewCompareUseCase(deps)
	err := uc.Execute(CompareOptions{})
	if err == nil {
		t.Fatal("expected error for missing production path, got nil")
	}
}

func TestCompareUseCase_Execute_ConfigLoadError(t *testing.T) {
	initI18nForTest(t)
	deps := newFullMockDeps()
	deps.ConfigLoader = &mocks.MockConfigLoader{
		LoadDepsFunc: func(configPath string) (*config.Deps, []string, error) {
			return nil, nil, fmt.Errorf("bad config")
		},
	}
	uc := NewCompareUseCase(deps)
	err := uc.Execute(CompareOptions{
		ConfigPath:     "does-not-exist.json",
		ProductionPath: "prod.yml",
	})
	if err == nil {
		t.Fatal("expected error for config load failure, got nil")
	}
}

func TestCompareUseCase_Execute_ProductionLoadError(t *testing.T) {
	initI18nForTest(t)
	deps := newFullMockDeps()
	deps.ConfigLoader = &mocks.MockConfigLoader{
		LoadDepsFunc: func(configPath string) (*config.Deps, []string, error) {
			return &config.Deps{
				Project:  config.Project{Name: "test"},
				Services: map[string]config.Service{},
			}, nil, nil
		},
	}
	uc := NewCompareUseCase(deps)
	// Production path must not exist or be invalid
	err := uc.Execute(CompareOptions{
		ConfigPath:     "ok.json",
		ProductionPath: filepath.Join(t.TempDir(), "nonexistent.yml"),
	})
	if err == nil {
		t.Fatal("expected error for production load failure, got nil")
	}
}

func TestCompareUseCase_Execute_WithWarnings(t *testing.T) {
	initI18nForTest(t)
	deps := newFullMockDeps()
	deps.ConfigLoader = &mocks.MockConfigLoader{
		LoadDepsFunc: func(configPath string) (*config.Deps, []string, error) {
			return &config.Deps{
				Project:  config.Project{Name: "test"},
				Services: map[string]config.Service{},
			}, []string{"some warning"}, nil
		},
	}
	uc := NewCompareUseCase(deps)
	// Will still fail on production load — just exercises the warnings path
	err := uc.Execute(CompareOptions{
		ConfigPath:     "ok.json",
		ProductionPath: "/nonexistent/prod.yml",
	})
	if err == nil {
		t.Fatal("expected error for invalid production path, got nil")
	}
}
