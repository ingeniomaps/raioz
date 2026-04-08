package app

import (
	"fmt"
	"os"

	"raioz/internal/config"
)

// getEnviron returns the current environment variables.
// Extracted as a package-level function to allow testing.
var getEnviron = os.Environ

// validateFastPreflight performs only critical preflight checks
func (uc *CIUseCase) validateFastPreflight() error {
	// Only check Docker (required for CI)
	if err := uc.deps.Validator.CheckDockerInstalled(); err != nil {
		return err
	}
	if err := uc.deps.Validator.CheckDockerRunning(); err != nil {
		return err
	}
	// Skip disk space, network, git checks for speed in CI
	return nil
}

// validateFast performs fast validation without compatibility checks
func (uc *CIUseCase) validateFast(deps *config.Deps) error {
	// Schema validation
	if err := uc.deps.Validator.ValidateSchema(deps); err != nil {
		return fmt.Errorf("schema validation failed: %w", err)
	}

	// Project validation
	if err := uc.deps.Validator.ValidateProject(deps); err != nil {
		return err
	}

	// Services validation
	if err := uc.deps.Validator.ValidateServices(deps); err != nil {
		return err
	}

	// Infra validation
	if err := uc.deps.Validator.ValidateInfra(deps); err != nil {
		return err
	}

	// Dependencies validation
	if err := uc.deps.Validator.ValidateDependencies(deps); err != nil {
		return err
	}

	// Skip compatibility checks for speed
	return nil
}

// checkWorkspacePermissions checks workspace permissions
func (uc *CIUseCase) checkWorkspacePermissions(workspacePath string) error {
	return uc.deps.Validator.CheckWorkspacePermissions(workspacePath)
}
