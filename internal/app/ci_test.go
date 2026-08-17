package app

import (
	"fmt"
	"strings"
	"testing"

	"raioz/internal/domain/models"
	"raioz/internal/mocks"
)

func TestNewCIUseCase(t *testing.T) {
	deps := newFullMockDeps()
	uc := NewCIUseCase(deps)
	if uc == nil {
		t.Fatal("expected non-nil CIUseCase")
	}
}

func TestCIUseCase_Execute_PreflightFails(t *testing.T) {
	initI18nForTest(t)
	deps := newFullMockDeps()
	deps.Validator = &mocks.MockValidator{
		CheckDockerInstalledFunc: func() error { return fmt.Errorf("no docker") },
	}
	uc := NewCIUseCase(deps)
	result, err := uc.Execute(CIOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Error("expected Success=false for preflight failure")
	}
	if len(result.Errors) == 0 {
		t.Error("expected errors recorded")
	}
}

func TestCIUseCase_Execute_PreflightDockerNotRunning(t *testing.T) {
	initI18nForTest(t)
	deps := newFullMockDeps()
	deps.Validator = &mocks.MockValidator{
		CheckDockerInstalledFunc: func() error { return nil },
		CheckDockerRunningFunc:   func() error { return fmt.Errorf("not running") },
	}
	uc := NewCIUseCase(deps)
	result, err := uc.Execute(CIOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Error("expected Success=false for docker not running")
	}
}

func TestCIUseCase_Execute_NoYAMLProject(t *testing.T) {
	initI18nForTest(t)
	deps := newFullMockDeps()
	deps.Validator = &mocks.MockValidator{}
	deps.ConfigLoader = &mocks.MockConfigLoader{
		LoadDepsFunc: func(configPath string) (*models.Deps, []string, error) {
			return nil, nil, fmt.Errorf(".raioz.json is no longer supported")
		},
	}
	uc := NewCIUseCase(deps)
	result, err := uc.Execute(CIOptions{OnlyValidate: true, ConfigPath: "raioz.json"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Error("expected Success=false without a raioz.yaml project")
	}
	if len(result.Errors) == 0 || !strings.Contains(result.Errors[0], "no longer supported") {
		t.Errorf("expected the loader's own message, got %v", result.Errors)
	}
}

func TestCIUseCase_Execute_LegacyLoadConfigFails(t *testing.T) {
	initI18nForTest(t)
	deps := newFullMockDeps()
	deps.Validator = &mocks.MockValidator{}
	deps.ConfigLoader = &mocks.MockConfigLoader{
		LoadDepsFunc: func(configPath string) (*models.Deps, []string, error) {
			return nil, nil, fmt.Errorf("cannot load")
		},
	}
	uc := NewCIUseCase(deps)
	result, err := uc.Execute(CIOptions{ConfigPath: "nope.json"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Error("expected Success=false on load failure")
	}
}

func TestCIUseCase_Execute_YAMLValidateOnly(t *testing.T) {
	initI18nForTest(t)
	deps := newFullMockDeps()
	deps.Validator = &mocks.MockValidator{}
	deps.ConfigLoader = &mocks.MockConfigLoader{
		LoadDepsFunc: func(configPath string) (*models.Deps, []string, error) {
			return &models.Deps{
				Project:       models.Project{Name: "yaml-proj"},
				Network:       models.NetworkConfig{Name: "net"},
				SchemaVersion: "2.0",
				SourceFormat:  models.SourceFormatYAML,
				Services:      map[string]models.Service{},
				Infra: map[string]models.InfraEntry{
					"redis": {Inline: &models.Infra{Image: "redis:7"}},
				},
			}, nil, nil
		},
	}
	uc := NewCIUseCase(deps)
	result, err := uc.Execute(CIOptions{
		OnlyValidate: true,
		ConfigPath:   "raioz.yaml",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Errorf("expected Success=true, got errors=%v", result.Errors)
	}
}

func TestCIUseCase_Execute_YAMLMissingImage(t *testing.T) {
	initI18nForTest(t)
	deps := newFullMockDeps()
	deps.Validator = &mocks.MockValidator{}
	deps.ConfigLoader = &mocks.MockConfigLoader{
		LoadDepsFunc: func(configPath string) (*models.Deps, []string, error) {
			return &models.Deps{
				Project:       models.Project{Name: "yaml-proj"},
				Network:       models.NetworkConfig{Name: "net"},
				SchemaVersion: "2.0",
				SourceFormat:  models.SourceFormatYAML,
				Services:      map[string]models.Service{},
				Infra: map[string]models.InfraEntry{
					"redis": {Inline: &models.Infra{Image: ""}},
				},
			}, nil, nil
		},
	}
	uc := NewCIUseCase(deps)
	result, err := uc.Execute(CIOptions{
		OnlyValidate: true,
		ConfigPath:   "raioz.yaml",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Error("expected Success=false for missing image")
	}
}
