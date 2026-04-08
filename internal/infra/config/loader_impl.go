package config

import (
	"raioz/internal/config"
	"raioz/internal/domain/interfaces"
)

// Ensure ConfigLoaderImpl implements interfaces.ConfigLoader
var _ interfaces.ConfigLoader = (*ConfigLoaderImpl)(nil)

// ConfigLoaderImpl is the concrete implementation of ConfigLoader
type ConfigLoaderImpl struct{}

// NewConfigLoader creates a new ConfigLoader implementation
func NewConfigLoader() interfaces.ConfigLoader {
	return &ConfigLoaderImpl{}
}

// LoadDeps loads dependencies configuration from a file
func (l *ConfigLoaderImpl) LoadDeps(configPath string) (*config.Deps, []string, error) {
	return config.LoadDeps(configPath)
}

// IsServiceEnabled checks if a service is enabled based on its configuration
func (l *ConfigLoaderImpl) IsServiceEnabled(svc config.Service, profile string, envVars map[string]string) bool {
	return config.IsServiceEnabled(svc, profile, envVars)
}

// ValidateFeatureFlags validates feature flags in the configuration
func (l *ConfigLoaderImpl) ValidateFeatureFlags(deps *config.Deps) error {
	return config.ValidateFeatureFlags(deps)
}

// FilterByProfile filters dependencies by a single profile
func (l *ConfigLoaderImpl) FilterByProfile(deps *config.Deps, profile string) *config.Deps {
	return config.FilterByProfile(deps, profile)
}

// FilterByProfiles filters dependencies by multiple profiles
func (l *ConfigLoaderImpl) FilterByProfiles(deps *config.Deps, profiles []string) *config.Deps {
	return config.FilterByProfiles(deps, profiles)
}

// FilterByFeatureFlags filters dependencies by feature flags
func (l *ConfigLoaderImpl) FilterByFeatureFlags(deps *config.Deps, profile string, envVars map[string]string) (*config.Deps, []string) {
	return config.FilterByFeatureFlags(deps, profile, envVars)
}

// FilterIgnoredServices filters out ignored services
func (l *ConfigLoaderImpl) FilterIgnoredServices(deps *config.Deps) (*config.Deps, []string, error) {
	return config.FilterIgnoredServices(deps)
}

// CheckIgnoredDependencies checks if ignored services are dependencies of active services
func (l *ConfigLoaderImpl) CheckIgnoredDependencies(deps *config.Deps, ignoredServices []string) map[string][]string {
	return config.CheckIgnoredDependencies(deps, ignoredServices)
}

// DetectMissingDependencies detects dependencies that are required but not defined
func (l *ConfigLoaderImpl) DetectMissingDependencies(deps *config.Deps, pathResolver func(string, config.Service) string) ([]config.MissingDependency, error) {
	return config.DetectMissingDependencies(deps, pathResolver)
}

// DetectDependencyConflicts detects conflicts between root and service dependencies
func (l *ConfigLoaderImpl) DetectDependencyConflicts(deps *config.Deps, pathResolver func(string, config.Service) string) ([]config.DependencyConflict, error) {
	return config.DetectDependencyConflicts(deps, pathResolver)
}

// FindServiceConfig finds and loads configuration from a service's .raioz.json
func (l *ConfigLoaderImpl) FindServiceConfig(servicePath string) (*config.Deps, string, error) {
	return config.FindServiceConfig(servicePath)
}
