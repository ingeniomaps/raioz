package interfaces

import (
	"context"
	models "raioz/internal/domain/models"
)

// ConfigLoader defines operations for loading configuration
type ConfigLoader interface {
	// LoadDeps loads dependencies configuration from a file
	LoadDeps(configPath string) (*models.Deps, []string, error)
	// IsServiceEnabled checks if a service is enabled based on its configuration
	IsServiceEnabled(svc models.Service, profile string, envVars map[string]string) bool
	// ValidateFeatureFlags validates feature flags in the configuration
	ValidateFeatureFlags(deps *models.Deps) error
	// FilterByProfile filters dependencies by a single profile
	FilterByProfile(deps *models.Deps, profile string) *models.Deps
	// FilterByProfiles filters dependencies by multiple profiles
	FilterByProfiles(deps *models.Deps, profiles []string) *models.Deps
	// FilterByFeatureFlags filters dependencies by feature flags
	FilterByFeatureFlags(deps *models.Deps, profile string, envVars map[string]string) (*models.Deps, []string)
	// FilterIgnoredServices filters out ignored services
	FilterIgnoredServices(deps *models.Deps) (*models.Deps, []string, error)
	// CheckIgnoredDependencies checks if ignored services are dependencies of active services
	CheckIgnoredDependencies(deps *models.Deps, ignoredServices []string) map[string][]string
	// DetectMissingDependencies detects dependencies that are required but not defined
	DetectMissingDependencies(deps *models.Deps, pathResolver func(string, models.Service) string) ([]models.MissingDependency, error)
	// DetectDependencyConflicts detects conflicts between root and service dependencies
	DetectDependencyConflicts(deps *models.Deps, pathResolver func(string, models.Service) string) ([]models.DependencyConflict, error)
	// FindServiceConfig finds and loads configuration from a service's .raioz.json
	FindServiceConfig(servicePath string) (*models.Deps, string, error)
}

// Validator defines operations for validating configuration
type Validator interface {
	// ValidateBeforeUp validates configuration before running up command
	ValidateBeforeUp(ctx interface{}, deps *models.Deps, ws interface{}) error
	// ValidateBeforeDown validates configuration before running down command
	ValidateBeforeDown(ctx interface{}, ws interface{}) error
	// All validates the entire configuration
	All(deps *models.Deps) error
	// CheckDockerInstalled checks if Docker is installed
	CheckDockerInstalled() error
	// CheckDockerRunning checks if Docker daemon is running
	CheckDockerRunning() error
	// ValidateSchema validates the configuration schema
	ValidateSchema(deps *models.Deps) error
	// ValidateProject validates the project configuration
	ValidateProject(deps *models.Deps) error
	// ValidateServices validates services configuration
	ValidateServices(deps *models.Deps) error
	// ValidateInfra validates infra configuration
	ValidateInfra(deps *models.Deps) error
	// ValidateDependencies validates dependencies configuration
	ValidateDependencies(deps *models.Deps) error
	// CheckWorkspacePermissions checks workspace permissions
	CheckWorkspacePermissions(workspacePath string) error
	// PreflightCheckWithContext runs preflight checks (Docker, Git, disk space)
	PreflightCheckWithContext(ctx context.Context) error
}
