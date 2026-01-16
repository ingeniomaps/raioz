package interfaces

import "raioz/internal/config"

// ConfigLoader defines operations for loading configuration
type ConfigLoader interface {
	// LoadDeps loads dependencies configuration from a file
	LoadDeps(configPath string) (*config.Deps, []string, error)
	// IsServiceEnabled checks if a service is enabled based on its configuration
	IsServiceEnabled(svc config.Service, profile string, envVars map[string]string) bool
}

// Validator defines operations for validating configuration
type Validator interface {
	// ValidateBeforeUp validates configuration before running up command
	ValidateBeforeUp(ctx interface{}, deps *config.Deps, ws interface{}) error
	// ValidateBeforeDown validates configuration before running down command
	ValidateBeforeDown(ctx interface{}, ws interface{}) error
	// All validates the entire configuration
	All(deps *config.Deps) error
}
