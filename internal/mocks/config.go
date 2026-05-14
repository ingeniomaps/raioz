package mocks

import (
	"context"

	"raioz/internal/domain/interfaces"
	"raioz/internal/domain/models"
)

// Compile-time checks
var _ interfaces.ConfigLoader = (*MockConfigLoader)(nil)
var _ interfaces.Validator = (*MockValidator)(nil)

// MockConfigLoader is a mock implementation of interfaces.ConfigLoader
type MockConfigLoader struct {
	LoadDepsFunc             func(configPath string) (*models.Deps, []string, error)
	IsServiceEnabledFunc     func(svc models.Service, profile string, envVars map[string]string) bool
	ValidateFeatureFlagsFunc func(deps *models.Deps) error
	FilterByProfileFunc      func(deps *models.Deps, profile string) *models.Deps
	FilterByProfilesFunc     func(deps *models.Deps, profiles []string) *models.Deps
	FilterByFeatureFlagsFunc func(
		deps *models.Deps, profile string, envVars map[string]string,
	) (*models.Deps, []string)
	FilterIgnoredServicesFunc     func(deps *models.Deps) (*models.Deps, []string, error)
	CheckIgnoredDependenciesFunc  func(deps *models.Deps, ignoredServices []string) map[string][]string
	DetectMissingDependenciesFunc func(
		deps *models.Deps, pathResolver func(string, models.Service) string,
	) ([]models.MissingDependency, error)
	DetectDependencyConflictsFunc func(
		deps *models.Deps, pathResolver func(string, models.Service) string,
	) ([]models.DependencyConflict, error)
	FindServiceConfigFunc func(servicePath string) (*models.Deps, string, error)
}

func (m *MockConfigLoader) LoadDeps(configPath string) (*models.Deps, []string, error) {
	if m.LoadDepsFunc != nil {
		return m.LoadDepsFunc(configPath)
	}
	return nil, nil, nil
}

func (m *MockConfigLoader) IsServiceEnabled(svc models.Service, profile string, envVars map[string]string) bool {
	if m.IsServiceEnabledFunc != nil {
		return m.IsServiceEnabledFunc(svc, profile, envVars)
	}
	return false
}

func (m *MockConfigLoader) ValidateFeatureFlags(deps *models.Deps) error {
	if m.ValidateFeatureFlagsFunc != nil {
		return m.ValidateFeatureFlagsFunc(deps)
	}
	return nil
}

func (m *MockConfigLoader) FilterByProfile(deps *models.Deps, profile string) *models.Deps {
	if m.FilterByProfileFunc != nil {
		return m.FilterByProfileFunc(deps, profile)
	}
	return nil
}

func (m *MockConfigLoader) FilterByProfiles(deps *models.Deps, profiles []string) *models.Deps {
	if m.FilterByProfilesFunc != nil {
		return m.FilterByProfilesFunc(deps, profiles)
	}
	return nil
}

func (m *MockConfigLoader) FilterByFeatureFlags(
	deps *models.Deps, profile string, envVars map[string]string,
) (*models.Deps, []string) {
	if m.FilterByFeatureFlagsFunc != nil {
		return m.FilterByFeatureFlagsFunc(deps, profile, envVars)
	}
	return nil, nil
}

func (m *MockConfigLoader) FilterIgnoredServices(deps *models.Deps) (*models.Deps, []string, error) {
	if m.FilterIgnoredServicesFunc != nil {
		return m.FilterIgnoredServicesFunc(deps)
	}
	return nil, nil, nil
}

func (m *MockConfigLoader) CheckIgnoredDependencies(deps *models.Deps, ignoredServices []string) map[string][]string {
	if m.CheckIgnoredDependenciesFunc != nil {
		return m.CheckIgnoredDependenciesFunc(deps, ignoredServices)
	}
	return nil
}

func (m *MockConfigLoader) DetectMissingDependencies(
	deps *models.Deps, pathResolver func(string, models.Service) string,
) ([]models.MissingDependency, error) {
	if m.DetectMissingDependenciesFunc != nil {
		return m.DetectMissingDependenciesFunc(deps, pathResolver)
	}
	return nil, nil
}

func (m *MockConfigLoader) DetectDependencyConflicts(
	deps *models.Deps, pathResolver func(string, models.Service) string,
) ([]models.DependencyConflict, error) {
	if m.DetectDependencyConflictsFunc != nil {
		return m.DetectDependencyConflictsFunc(deps, pathResolver)
	}
	return nil, nil
}

func (m *MockConfigLoader) FindServiceConfig(servicePath string) (*models.Deps, string, error) {
	if m.FindServiceConfigFunc != nil {
		return m.FindServiceConfigFunc(servicePath)
	}
	return nil, "", nil
}

// MockValidator is a mock implementation of interfaces.Validator
type MockValidator struct {
	ValidateBeforeUpFunc          func(ctx interface{}, deps *models.Deps, ws interface{}) error
	ValidateBeforeDownFunc        func(ctx interface{}, ws interface{}) error
	AllFunc                       func(deps *models.Deps) error
	CheckDockerInstalledFunc      func() error
	CheckDockerRunningFunc        func() error
	ValidateSchemaFunc            func(deps *models.Deps) error
	ValidateProjectFunc           func(deps *models.Deps) error
	ValidateServicesFunc          func(deps *models.Deps) error
	ValidateInfraFunc             func(deps *models.Deps) error
	ValidateDependenciesFunc      func(deps *models.Deps) error
	CheckWorkspacePermissionsFunc func(workspacePath string) error
	PreflightCheckWithContextFunc func(ctx context.Context) error
}

func (m *MockValidator) ValidateBeforeUp(ctx interface{}, deps *models.Deps, ws interface{}) error {
	if m.ValidateBeforeUpFunc != nil {
		return m.ValidateBeforeUpFunc(ctx, deps, ws)
	}
	return nil
}

func (m *MockValidator) ValidateBeforeDown(ctx interface{}, ws interface{}) error {
	if m.ValidateBeforeDownFunc != nil {
		return m.ValidateBeforeDownFunc(ctx, ws)
	}
	return nil
}

func (m *MockValidator) All(deps *models.Deps) error {
	if m.AllFunc != nil {
		return m.AllFunc(deps)
	}
	return nil
}

func (m *MockValidator) CheckDockerInstalled() error {
	if m.CheckDockerInstalledFunc != nil {
		return m.CheckDockerInstalledFunc()
	}
	return nil
}

func (m *MockValidator) CheckDockerRunning() error {
	if m.CheckDockerRunningFunc != nil {
		return m.CheckDockerRunningFunc()
	}
	return nil
}

func (m *MockValidator) ValidateSchema(deps *models.Deps) error {
	if m.ValidateSchemaFunc != nil {
		return m.ValidateSchemaFunc(deps)
	}
	return nil
}

func (m *MockValidator) ValidateProject(deps *models.Deps) error {
	if m.ValidateProjectFunc != nil {
		return m.ValidateProjectFunc(deps)
	}
	return nil
}

func (m *MockValidator) ValidateServices(deps *models.Deps) error {
	if m.ValidateServicesFunc != nil {
		return m.ValidateServicesFunc(deps)
	}
	return nil
}

func (m *MockValidator) ValidateInfra(deps *models.Deps) error {
	if m.ValidateInfraFunc != nil {
		return m.ValidateInfraFunc(deps)
	}
	return nil
}

func (m *MockValidator) ValidateDependencies(deps *models.Deps) error {
	if m.ValidateDependenciesFunc != nil {
		return m.ValidateDependenciesFunc(deps)
	}
	return nil
}

func (m *MockValidator) CheckWorkspacePermissions(workspacePath string) error {
	if m.CheckWorkspacePermissionsFunc != nil {
		return m.CheckWorkspacePermissionsFunc(workspacePath)
	}
	return nil
}

func (m *MockValidator) PreflightCheckWithContext(ctx context.Context) error {
	if m.PreflightCheckWithContextFunc != nil {
		return m.PreflightCheckWithContextFunc(ctx)
	}
	return nil
}
