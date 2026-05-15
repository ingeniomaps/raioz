package config

import (
	"raioz/internal/config"
	"raioz/internal/domain/interfaces"
	"raioz/internal/domain/models"
)

// Ensure ConfigLoaderImpl implements interfaces.ConfigLoader
var _ interfaces.ConfigLoader = (*ConfigLoaderImpl)(nil)

// ConfigLoaderImpl is the concrete implementation of ConfigLoader
type ConfigLoaderImpl struct{}

// NewConfigLoader creates a new ConfigLoader implementation
func NewConfigLoader() interfaces.ConfigLoader {
	return &ConfigLoaderImpl{}
}

// LoadDeps loads dependencies configuration from a file.
// Supports raioz.yaml (new), .raioz.json (legacy), and auto-detect mode (no file).
func (l *ConfigLoaderImpl) LoadDeps(configPath string) (*models.Deps, []string, error) {
	// Auto-detect mode: no config file found
	if configPath == ":auto:" {
		deps, err := config.AutoDetect(".")
		if err != nil {
			return nil, nil, err
		}
		return deps, nil, nil
	}
	if config.IsYAMLConfig(configPath) {
		return config.LoadDepsFromYAML(configPath)
	}
	// JSON path. LoadDeps emits its own deprecation banner (ADR-038).
	return config.LoadDeps(configPath)
}

// IsServiceEnabled checks if a service is enabled based on its configuration
func (l *ConfigLoaderImpl) IsServiceEnabled(
	svc models.Service, profile string, envVars map[string]string,
) bool {
	return config.IsServiceEnabled(svc, profile, envVars)
}

// ValidateFeatureFlags validates feature flags in the configuration
func (l *ConfigLoaderImpl) ValidateFeatureFlags(deps *models.Deps) error {
	return config.ValidateFeatureFlags(deps)
}

// FilterByProfile filters dependencies by a single profile
func (l *ConfigLoaderImpl) FilterByProfile(deps *models.Deps, profile string) *models.Deps {
	return config.FilterByProfile(deps, profile)
}

// FilterByProfiles filters dependencies by multiple profiles
func (l *ConfigLoaderImpl) FilterByProfiles(deps *models.Deps, profiles []string) *models.Deps {
	return config.FilterByProfiles(deps, profiles)
}

// FilterByFeatureFlags filters dependencies by feature flags
func (l *ConfigLoaderImpl) FilterByFeatureFlags(
	deps *models.Deps, profile string, envVars map[string]string,
) (*models.Deps, []string) {
	return config.FilterByFeatureFlags(deps, profile, envVars)
}

// FilterIgnoredServices filters out ignored services
func (l *ConfigLoaderImpl) FilterIgnoredServices(deps *models.Deps) (*models.Deps, []string, error) {
	return config.FilterIgnoredServices(deps)
}

// CheckIgnoredDependencies checks if ignored services are dependencies of active services
func (l *ConfigLoaderImpl) CheckIgnoredDependencies(deps *models.Deps, ignoredServices []string) map[string][]string {
	return config.CheckIgnoredDependencies(deps, ignoredServices)
}

// DetectMissingDependencies detects dependencies that are required but not defined
func (l *ConfigLoaderImpl) DetectMissingDependencies(
	deps *models.Deps, pathResolver func(string, models.Service) string,
) ([]models.MissingDependency, error) {
	return config.DetectMissingDependencies(deps, pathResolver)
}

// DetectDependencyConflicts detects conflicts between root and service deps
func (l *ConfigLoaderImpl) DetectDependencyConflicts(
	deps *models.Deps, pathResolver func(string, models.Service) string,
) ([]models.DependencyConflict, error) {
	return config.DetectDependencyConflicts(deps, pathResolver)
}

// FindServiceConfig finds and loads configuration from a service's .raioz.json
func (l *ConfigLoaderImpl) FindServiceConfig(servicePath string) (*models.Deps, string, error) {
	return config.FindServiceConfig(servicePath)
}
