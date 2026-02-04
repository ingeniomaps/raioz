package initcase

import (
	"encoding/json"
	"fmt"
	"os"

	"raioz/internal/config"
	"raioz/internal/errors"
	"raioz/internal/validate"
)

// createConfig creates a minimal valid configuration
func (uc *UseCase) createConfig(projectName string, networkName string) (*config.Deps, error) {
	deps := &config.Deps{
		SchemaVersion: "1.0",
		Network:       config.NetworkConfig{Name: networkName, IsObject: false},
		Project: config.Project{
			Name: projectName,
		},
		Services: make(map[string]config.Service),
		Infra:    make(map[string]config.Infra),
		Env: config.EnvConfig{
			UseGlobal: true,
			Files:     []string{"global"},
		},
	}

	// Validate configuration (only schema and project, not services)
	// This allows creating a minimal config file that can be extended later
	if err := validate.ValidateSchema(deps); err != nil {
		return nil, errors.New(
			errors.ErrCodeInvalidConfig,
			"Generated configuration is invalid",
		).WithError(err)
	}
	if err := validate.ValidateProject(deps); err != nil {
		return nil, errors.New(
			errors.ErrCodeInvalidConfig,
			"Generated configuration is invalid",
		).WithError(err)
	}

	return deps, nil
}

// writeConfigFile writes the configuration to a file
func (uc *UseCase) writeConfigFile(outputPath string, deps *config.Deps) error {
	// Marshal configuration to JSON
	data, err := json.MarshalIndent(deps, "", "  ")
	if err != nil {
		return errors.New(
			errors.ErrCodeInvalidConfig,
			"Failed to marshal configuration",
		).WithError(err)
	}

	// Write file
	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return errors.New(
			errors.ErrCodeWorkspaceError,
			fmt.Sprintf("Failed to write .raioz.json to %s", outputPath),
		).WithError(err).WithContext("output_path", outputPath)
	}

	return nil
}
