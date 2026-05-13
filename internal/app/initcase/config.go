package initcase

import (
	"encoding/json"
	"fmt"
	"os"

	"raioz/internal/domain/models"
	"raioz/internal/errors"
	"raioz/internal/validate"
)

// createConfig creates a valid configuration from wizard inputs
func (uc *UseCase) createConfig(
	projectName string,
	networkName string,
	services []serviceResult,
	infra map[string]models.InfraEntry,
) (*models.Deps, error) {
	svcMap := make(map[string]models.Service)
	for _, svc := range services {
		svcMap[svc.Name] = models.Service{
			Source: svc.Source,
			Docker: svc.Docker,
		}
	}

	infraMap := make(map[string]models.InfraEntry)
	if infra != nil {
		infraMap = infra
	}

	deps := &models.Deps{
		SchemaVersion: "1.0",
		Network:       models.NetworkConfig{Name: networkName, IsObject: false},
		Project: models.Project{
			Name: projectName,
		},
		Services: svcMap,
		Infra:    infraMap,
		Env: models.EnvConfig{
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
func (uc *UseCase) writeConfigFile(outputPath string, deps *models.Deps) error {
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
