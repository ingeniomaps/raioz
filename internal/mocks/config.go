package mocks

import (
	"context"

	"raioz/internal/config"
	"raioz/internal/domain/interfaces"
)

// Compile-time checks
var _ interfaces.ConfigLoader = (*MockConfigLoader)(nil)
var _ interfaces.Validator = (*MockValidator)(nil)

// MockConfigLoader is a mock implementation of interfaces.ConfigLoader
type MockConfigLoader struct {
	LoadDepsFunc             func(configPath string) (*config.Deps, []string, error)
	IsServiceEnabledFunc     func(svc config.Service, profile string, envVars map[string]string) bool
	ValidateFeatureFlagsFunc func(deps *config.Deps) error
	FilterByProfileFunc      func(deps *config.Deps, profile string) *config.Deps
	FilterByProfilesFunc     func(deps *config.Deps, profiles []string) *config.Deps
	FilterByFeatureFlagsFunc func(
		deps *config.Deps, profile string, envVars map[string]string,
	) (*config.Deps, []string)
	FilterIgnoredServicesFunc     func(deps *config.Deps) (*config.Deps, []string, error)
	CheckIgnoredDependenciesFunc  func(deps *config.Deps, ignoredServices []string) map[string][]string
	DetectMissingDependenciesFunc func(
		deps *config.Deps, pathResolver func(string, config.Service) string,
	) ([]config.MissingDependency, error)
	DetectDependencyConflictsFunc func(
		deps *config.Deps, pathResolver func(string, config.Service) string,
	) ([]config.DependencyConflict, error)
	FindServiceConfigFunc func(servicePath string) (*config.Deps, string, error)
}

func (m *MockConfigLoader) LoadDeps(configPath string) (*config.Deps, []string, error) {
	if m.LoadDepsFunc != nil {
		return m.LoadDepsFunc(configPath)
	}
	return nil, nil, nil
}

func (m *MockConfigLoader) IsServiceEnabled(svc config.Service, profile string, envVars map[string]string) bool {
	if m.IsServiceEnabledFunc != nil {
		return m.IsServiceEnabledFunc(svc, profile, envVars)
	}
	return false
}

func (m *MockConfigLoader) ValidateFeatureFlags(deps *config.Deps) error {
	if m.ValidateFeatureFlagsFunc != nil {
		return m.ValidateFeatureFlagsFunc(deps)
	}
	return nil
}

func (m *MockConfigLoader) FilterByProfile(deps *config.Deps, profile string) *config.Deps {
	if m.FilterByProfileFunc != nil {
		return m.FilterByProfileFunc(deps, profile)
	}
	return nil
}

func (m *MockConfigLoader) FilterByProfiles(deps *config.Deps, profiles []string) *config.Deps {
	if m.FilterByProfilesFunc != nil {
		return m.FilterByProfilesFunc(deps, profiles)
	}
	return nil
}

func (m *MockConfigLoader) FilterByFeatureFlags(
	deps *config.Deps, profile string, envVars map[string]string,
) (*config.Deps, []string) {
	if m.FilterByFeatureFlagsFunc != nil {
		return m.FilterByFeatureFlagsFunc(deps, profile, envVars)
	}
	return nil, nil
}

func (m *MockConfigLoader) FilterIgnoredServices(deps *config.Deps) (*config.Deps, []string, error) {
	if m.FilterIgnoredServicesFunc != nil {
		return m.FilterIgnoredServicesFunc(deps)
	}
	return nil, nil, nil
}

func (m *MockConfigLoader) CheckIgnoredDependencies(deps *config.Deps, ignoredServices []string) map[string][]string {
	if m.CheckIgnoredDependenciesFunc != nil {
		return m.CheckIgnoredDependenciesFunc(deps, ignoredServices)
	}
	return nil
}

func (m *MockConfigLoader) DetectMissingDependencies(
	deps *config.Deps, pathResolver func(string, config.Service) string,
) ([]config.MissingDependency, error) {
	if m.DetectMissingDependenciesFunc != nil {
		return m.DetectMissingDependenciesFunc(deps, pathResolver)
	}
	return nil, nil
}

func (m *MockConfigLoader) DetectDependencyConflicts(
	deps *config.Deps, pathResolver func(string, config.Service) string,
) ([]config.DependencyConflict, error) {
	if m.DetectDependencyConflictsFunc != nil {
		return m.DetectDependencyConflictsFunc(deps, pathResolver)
	}
	return nil, nil
}

func (m *MockConfigLoader) FindServiceConfig(servicePath string) (*config.Deps, string, error) {
	if m.FindServiceConfigFunc != nil {
		return m.FindServiceConfigFunc(servicePath)
	}
	return nil, "", nil
}

// MockValidator is a mock implementation of interfaces.Validator
type MockValidator struct {
	ValidateBeforeUpFunc          func(ctx interface{}, deps *config.Deps, ws interface{}) error
	ValidateBeforeDownFunc        func(ctx interface{}, ws interface{}) error
	AllFunc                       func(deps *config.Deps) error
	CheckDockerInstalledFunc      func() error
	CheckDockerRunningFunc        func() error
	ValidateSchemaFunc            func(deps *config.Deps) error
	ValidateProjectFunc           func(deps *config.Deps) error
	ValidateServicesFunc          func(deps *config.Deps) error
	ValidateInfraFunc             func(deps *config.Deps) error
	ValidateDependenciesFunc      func(deps *config.Deps) error
	CheckWorkspacePermissionsFunc func(workspacePath string) error
	PreflightCheckWithContextFunc func(ctx context.Context) error
}

func (m *MockValidator) ValidateBeforeUp(ctx interface{}, deps *config.Deps, ws interface{}) error {
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

func (m *MockValidator) All(deps *config.Deps) error {
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

func (m *MockValidator) ValidateSchema(deps *config.Deps) error {
	if m.ValidateSchemaFunc != nil {
		return m.ValidateSchemaFunc(deps)
	}
	return nil
}

func (m *MockValidator) ValidateProject(deps *config.Deps) error {
	if m.ValidateProjectFunc != nil {
		return m.ValidateProjectFunc(deps)
	}
	return nil
}

func (m *MockValidator) ValidateServices(deps *config.Deps) error {
	if m.ValidateServicesFunc != nil {
		return m.ValidateServicesFunc(deps)
	}
	return nil
}

func (m *MockValidator) ValidateInfra(deps *config.Deps) error {
	if m.ValidateInfraFunc != nil {
		return m.ValidateInfraFunc(deps)
	}
	return nil
}

func (m *MockValidator) ValidateDependencies(deps *config.Deps) error {
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
