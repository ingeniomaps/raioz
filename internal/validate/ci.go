package validate

import "raioz/internal/domain/models"

// CheckDockerInstalled verifies that Docker is installed (exported for CI)
func CheckDockerInstalled() error {
	return checkDockerInstalled()
}

// CheckDockerRunning verifies that Docker daemon is running (exported for CI)
func CheckDockerRunning() error {
	return checkDockerRunning()
}

// ValidateSchema validates only the schema (exported for CI)
func ValidateSchema(deps *models.Deps) error {
	return validateSchema(deps)
}

// ValidateProject validates only the project (exported for CI)
func ValidateProject(deps *models.Deps) error {
	return validateProject(deps)
}

// ValidateServices validates only services (exported for CI)
func ValidateServices(deps *models.Deps) error {
	return validateServices(deps)
}

// ValidateInfra validates only infra (exported for CI)
func ValidateInfra(deps *models.Deps) error {
	return validateInfra(deps)
}

// ValidateDependencies validates only dependencies (exported for CI)
func ValidateDependencies(deps *models.Deps) error {
	return validateDependencies(deps)
}
