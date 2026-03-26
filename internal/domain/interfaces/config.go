package interfaces

import (
	"context"
	"raioz/internal/config"
)

// ConfigLoader defines operations for loading configuration
type ConfigLoader interface {
	// LoadDeps loads dependencies configuration from a file
	LoadDeps(configPath string) (*config.Deps, []string, error)
	// IsServiceEnabled checks if a service is enabled based on its configuration
	IsServiceEnabled(svc config.Service, profile string, envVars map[string]string) bool
	// ValidateFeatureFlags validates feature flags in the configuration
	ValidateFeatureFlags(deps *config.Deps) error
	// FilterByProfile filters dependencies by a single profile
	FilterByProfile(deps *config.Deps, profile string) *config.Deps
	// FilterByProfiles filters dependencies by multiple profiles
	FilterByProfiles(deps *config.Deps, profiles []string) *config.Deps
	// FilterByFeatureFlags filters dependencies by feature flags
	FilterByFeatureFlags(deps *config.Deps, profile string, envVars map[string]string) (*config.Deps, []string)
	// FilterIgnoredServices filters out ignored services
	FilterIgnoredServices(deps *config.Deps) (*config.Deps, []string, error)
	// CheckIgnoredDependencies checks if ignored services are dependencies of active services
	CheckIgnoredDependencies(deps *config.Deps, ignoredServices []string) map[string][]string
	// DetectMissingDependencies detects dependencies that are required but not defined
	DetectMissingDependencies(deps *config.Deps, pathResolver func(string, config.Service) string) ([]config.MissingDependency, error)
	// DetectDependencyConflicts detects conflicts between root and service dependencies
	DetectDependencyConflicts(deps *config.Deps, pathResolver func(string, config.Service) string) ([]config.DependencyConflict, error)
	// FindServiceConfig finds and loads configuration from a service's .raioz.json
	FindServiceConfig(servicePath string) (*config.Deps, string, error)
}

// Validator defines operations for validating configuration
type Validator interface {
	// ValidateBeforeUp validates configuration before running up command
	ValidateBeforeUp(ctx interface{}, deps *config.Deps, ws interface{}) error
	// ValidateBeforeDown validates configuration before running down command
	ValidateBeforeDown(ctx interface{}, ws interface{}) error
	// All validates the entire configuration
	All(deps *config.Deps) error
	// CheckDockerInstalled checks if Docker is installed
	CheckDockerInstalled() error
	// CheckDockerRunning checks if Docker daemon is running
	CheckDockerRunning() error
	// ValidateSchema validates the configuration schema
	ValidateSchema(deps *config.Deps) error
	// ValidateProject validates the project configuration
	ValidateProject(deps *config.Deps) error
	// ValidateServices validates services configuration
	ValidateServices(deps *config.Deps) error
	// ValidateInfra validates infra configuration
	ValidateInfra(deps *config.Deps) error
	// ValidateDependencies validates dependencies configuration
	ValidateDependencies(deps *config.Deps) error
	// CheckWorkspacePermissions checks workspace permissions
	CheckWorkspacePermissions(workspacePath string) error
	// PreflightCheckWithContext runs preflight checks (Docker, Git, disk space)
	PreflightCheckWithContext(ctx context.Context) error
}
