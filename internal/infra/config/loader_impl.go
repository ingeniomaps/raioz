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
