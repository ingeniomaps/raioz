package app

import (
	"fmt"
	"testing"

	"raioz/internal/config"
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

func TestCIUseCase_Execute_LegacyValidateOnly(t *testing.T) {
	initI18nForTest(t)
	deps := newFullMockDeps()
	deps.Validator = &mocks.MockValidator{}
	deps.ConfigLoader = &mocks.MockConfigLoader{
		LoadDepsFunc: func(configPath string) (*config.Deps, []string, error) {
			return &config.Deps{
				Project:       config.Project{Name: "test"},
				SchemaVersion: "1.0",
				Services:      map[string]config.Service{},
				Infra:         map[string]config.InfraEntry{},
			}, nil, nil
		},
	}
	uc := NewCIUseCase(deps)
	result, err := uc.Execute(CIOptions{
		OnlyValidate: true,
		ConfigPath:   "raioz.json",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Errorf("expected Success=true, errors=%v", result.Errors)
	}
}

func TestCIUseCase_Execute_LegacyLoadConfigFails(t *testing.T) {
	initI18nForTest(t)
	deps := newFullMockDeps()
	deps.Validator = &mocks.MockValidator{}
	deps.ConfigLoader = &mocks.MockConfigLoader{
		LoadDepsFunc: func(configPath string) (*config.Deps, []string, error) {
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
		LoadDepsFunc: func(configPath string) (*config.Deps, []string, error) {
			return &config.Deps{
				Project:       config.Project{Name: "yaml-proj"},
				Network:       config.NetworkConfig{Name: "net"},
				SchemaVersion: "2.0",
				Services:      map[string]config.Service{},
				Infra: map[string]config.InfraEntry{
					"redis": {Inline: &config.Infra{Image: "redis:7"}},
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
		LoadDepsFunc: func(configPath string) (*config.Deps, []string, error) {
			return &config.Deps{
				Project:       config.Project{Name: "yaml-proj"},
				Network:       config.NetworkConfig{Name: "net"},
				SchemaVersion: "2.0",
				Services:      map[string]config.Service{},
				Infra: map[string]config.InfraEntry{
					"redis": {Inline: &config.Infra{Image: ""}},
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

func TestCIUseCase_validateFast(t *testing.T) {
	initI18nForTest(t)
	deps := newFullMockDeps()
	uc := NewCIUseCase(deps)
	cfgDeps := &config.Deps{
		Project: config.Project{Name: "test"},
	}
	if err := uc.validateFast(cfgDeps); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCIUseCase_validateFast_SchemaError(t *testing.T) {
	initI18nForTest(t)
	deps := newFullMockDeps()
	deps.Validator = &mocks.MockValidator{
		ValidateSchemaFunc: func(d *config.Deps) error { return fmt.Errorf("bad schema") },
	}
	uc := NewCIUseCase(deps)
	if err := uc.validateFast(&config.Deps{}); err == nil {
		t.Error("expected error for bad schema")
	}
}

func TestCIUseCase_checkWorkspacePermissions(t *testing.T) {
	deps := newFullMockDeps()
	uc := NewCIUseCase(deps)
	if err := uc.checkWorkspacePermissions("/tmp"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
