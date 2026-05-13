package app

import (
	"context"
	"fmt"
	"testing"

	"raioz/internal/domain/interfaces"
	"raioz/internal/domain/models"
	"raioz/internal/mocks"
	"raioz/internal/workspace"
)

func TestCIUseCase_executeLegacy_FeatureFlagsFail(t *testing.T) {
	initI18nForTest(t)
	deps := newFullMockDeps()
	deps.ConfigLoader = &mocks.MockConfigLoader{
		LoadDepsFunc: func(configPath string) (*models.Deps, []string, error) {
			return &models.Deps{
				Project:  models.Project{Name: "test"},
				Services: map[string]models.Service{},
				Infra:    map[string]models.InfraEntry{},
			}, nil, nil
		},
		ValidateFeatureFlagsFunc: func(d *models.Deps) error {
			return fmt.Errorf("bad flags")
		},
	}
	uc := NewCIUseCase(deps)
	result := &CIResult{Validations: []ValidationResult{}, Errors: []string{}, Warnings: []string{}}
	got, err := uc.executeLegacy(CIOptions{ConfigPath: "raioz.json"}, result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Success {
		t.Error("expected failure due to feature flags error")
	}
}

func TestCIUseCase_executeLegacy_ValidateOnly(t *testing.T) {
	initI18nForTest(t)
	deps := newFullMockDeps()
	deps.ConfigLoader = &mocks.MockConfigLoader{
		LoadDepsFunc: func(configPath string) (*models.Deps, []string, error) {
			return &models.Deps{
				Project:  models.Project{Name: "test"},
				Services: map[string]models.Service{},
				Infra:    map[string]models.InfraEntry{},
			}, nil, nil
		},
	}
	uc := NewCIUseCase(deps)
	result := &CIResult{Validations: []ValidationResult{}, Errors: []string{}, Warnings: []string{}}
	got, err := uc.executeLegacy(CIOptions{ConfigPath: "raioz.json", OnlyValidate: true}, result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.Success {
		t.Errorf("expected success, got errors: %v", got.Errors)
	}
}

func TestCIUseCase_executeLegacy_ValidationFails(t *testing.T) {
	initI18nForTest(t)
	deps := newFullMockDeps()
	deps.ConfigLoader = &mocks.MockConfigLoader{
		LoadDepsFunc: func(configPath string) (*models.Deps, []string, error) {
			return &models.Deps{
				Project:  models.Project{Name: "test"},
				Services: map[string]models.Service{},
				Infra:    map[string]models.InfraEntry{},
			}, nil, nil
		},
	}
	deps.Validator = &mocks.MockValidator{
		ValidateSchemaFunc: func(d *models.Deps) error {
			return fmt.Errorf("schema bad")
		},
	}
	uc := NewCIUseCase(deps)
	result := &CIResult{Validations: []ValidationResult{}, Errors: []string{}, Warnings: []string{}}
	got, err := uc.executeLegacy(CIOptions{ConfigPath: "raioz.json"}, result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Success {
		t.Error("expected failure due to validation error")
	}
}

func TestCIUseCase_executeLegacy_EphemeralWorkspace(t *testing.T) {
	initI18nForTest(t)
	tmpDir := t.TempDir()
	deps := newFullMockDeps()
	deps.ConfigLoader = &mocks.MockConfigLoader{
		LoadDepsFunc: func(configPath string) (*models.Deps, []string, error) {
			return &models.Deps{
				Project:  models.Project{Name: "test"},
				Services: map[string]models.Service{},
				Infra:    map[string]models.InfraEntry{},
			}, nil, nil
		},
		FilterByFeatureFlagsFunc: func(d *models.Deps, profile string, envVars map[string]string) (*models.Deps, []string) {
			return d, nil
		},
	}
	deps.Workspace = &mocks.MockWorkspaceManager{
		ResolveFunc: func(name string) (*workspace.Workspace, error) {
			return &workspace.Workspace{Root: tmpDir, ServicesDir: tmpDir + "/services"}, nil
		},
		GetBaseDirFromWorkspaceFunc: func(ws *workspace.Workspace) string { return tmpDir },
		GetComposePathFunc: func(ws *workspace.Workspace) string {
			return tmpDir + "/compose.yml"
		},
		GetStatePathFunc: func(ws *workspace.Workspace) string {
			return tmpDir + "/state.json"
		},
	}
	deps.DockerRunner = &mocks.MockDockerRunner{
		ValidatePortsFunc: func(d *models.Deps, baseDir string, projectName string) ([]interfaces.PortConflict, error) {
			return nil, nil
		},
		ValidateAllImagesFunc: func(d *models.Deps) error { return nil },
		EnsureNetworkWithConfigAndContextFunc: func(ctx context.Context, name string, subnet string, _ map[string]string, ask bool) error {
			return nil
		},
		ExtractNamedVolumesFunc: func(volumes []string) ([]string, error) { return nil, nil },
		GenerateComposeFunc: func(d *models.Deps, ws *workspace.Workspace, projectDir string) (string, []string, error) {
			return tmpDir + "/compose.yml", nil, nil
		},
		UpFunc: func(composePath string) error { return nil },
	}
	deps.StateManager = &mocks.MockStateManager{
		SaveFunc: func(ws *workspace.Workspace, d *models.Deps) error { return nil },
	}

	uc := NewCIUseCase(deps)
	result := &CIResult{Validations: []ValidationResult{}, Errors: []string{}, Warnings: []string{}}
	got, err := uc.executeLegacy(CIOptions{
		ConfigPath: tmpDir + "/raioz.json",
		Ephemeral:  true,
		JobID:      "42",
	}, result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.Success {
		t.Errorf("expected success, got errors: %v", got.Errors)
	}
	if got.Workspace == "" {
		t.Error("expected workspace name for ephemeral run")
	}
}

func TestCIUseCase_executeLegacy_LockFails(t *testing.T) {
	initI18nForTest(t)
	tmpDir := t.TempDir()
	deps := newFullMockDeps()
	deps.ConfigLoader = &mocks.MockConfigLoader{
		LoadDepsFunc: func(configPath string) (*models.Deps, []string, error) {
			return &models.Deps{
				Project:  models.Project{Name: "test"},
				Services: map[string]models.Service{},
				Infra:    map[string]models.InfraEntry{},
			}, nil, nil
		},
		FilterByFeatureFlagsFunc: func(d *models.Deps, profile string, envVars map[string]string) (*models.Deps, []string) {
			return d, nil
		},
	}
	deps.Workspace = &mocks.MockWorkspaceManager{
		ResolveFunc: func(name string) (*workspace.Workspace, error) {
			return &workspace.Workspace{Root: tmpDir}, nil
		},
	}
	deps.LockManager = &mocks.MockLockManager{
		AcquireFunc: func(ws *workspace.Workspace) (interfaces.Lock, error) {
			return nil, fmt.Errorf("lock fail")
		},
	}
	uc := NewCIUseCase(deps)
	result := &CIResult{Validations: []ValidationResult{}, Errors: []string{}, Warnings: []string{}}
	got, err := uc.executeLegacy(CIOptions{ConfigPath: tmpDir + "/raioz.json"}, result)
	// executeLegacy absorbs executeSetup errors into result.Errors (returns nil error)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Errors) == 0 {
		t.Fatal("expected lock failure recorded in result.Errors")
	}
}
