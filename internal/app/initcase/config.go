package initcase

import (
	"encoding/json"
	"fmt"
	"os"

	"raioz/internal/config"
	"raioz/internal/errors"
	"raioz/internal/validate"
)

// createConfig creates a valid configuration from wizard inputs
func (uc *UseCase) createConfig(
	projectName string,
	networkName string,
	services []serviceResult,
	infra map[string]config.InfraEntry,
) (*config.Deps, error) {
	svcMap := make(map[string]config.Service)
	for _, svc := range services {
		svcMap[svc.Name] = config.Service{
			Source: svc.Source,
			Docker: svc.Docker,
		}
	}

	infraMap := make(map[string]config.InfraEntry)
	if infra != nil {
		infraMap = infra
	}

	deps := &config.Deps{
		SchemaVersion: "1.0",
		Network:       config.NetworkConfig{Name: networkName, IsObject: false},
		Project: config.Project{
			Name: projectName,
		},
		Services: svcMap,
		Infra:    infraMap,
		Env: config.EnvConfig{
			UseGlobal: true,
			Files:     []string{"global", fmt.Sprintf("projects/%s", projectName)},
		},
	}

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
	data, err := json.MarshalIndent(deps, "", "  ")
	if err != nil {
		return errors.New(
			errors.ErrCodeInvalidConfig,
			"Failed to marshal configuration",
		).WithError(err)
	}

	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return errors.New(
			errors.ErrCodeWorkspaceError,
			fmt.Sprintf("Failed to write .raioz.json to %s", outputPath),
		).WithError(err).WithContext("output_path", outputPath)
	}

	return nil
}
