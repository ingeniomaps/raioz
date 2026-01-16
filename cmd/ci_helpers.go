package cmd

import (
	"fmt"
	"os"

	"raioz/internal/config"
	"raioz/internal/docker"
	"raioz/internal/validate"
	"raioz/internal/workspace"
)

// validateFastPreflight performs only critical preflight checks
func validateFastPreflight() error {
	// Only check Docker (required for CI)
	if err := validate.CheckDockerInstalled(); err != nil {
		return err
	}
	if err := validate.CheckDockerRunning(); err != nil {
		return err
	}
	// Skip disk space, network, git checks for speed in CI
	return nil
}

// validateFast performs fast validation without compatibility checks
func validateFast(deps *config.Deps) error {
	// Schema validation
	if err := validate.ValidateSchema(deps); err != nil {
		return fmt.Errorf("schema validation failed: %w", err)
	}

	// Project validation
	if err := validate.ValidateProject(deps); err != nil {
		return err
	}

	// Services validation
	if err := validate.ValidateServices(deps); err != nil {
		return err
	}

	// Infra validation
	if err := validate.ValidateInfra(deps); err != nil {
		return err
	}

	// Dependencies validation
	if err := validate.ValidateDependencies(deps); err != nil {
		return err
	}

	// Skip compatibility checks for speed
	return nil
}

// cleanupEphemeralEnvironment cleans up an ephemeral CI environment
func cleanupEphemeralEnvironment(ws *workspace.Workspace, projectName string) {
	composePath := workspace.GetComposePath(ws)

	// Stop services if compose file exists
	if _, err := os.Stat(composePath); err == nil {
		if err := docker.Down(composePath); err != nil {
			// Log but don't fail
			fmt.Fprintf(os.Stderr, "Warning: Failed to stop ephemeral environment: %v\n", err)
		}
	}

	// Remove state file
	statePath := workspace.GetStatePath(ws)
	if err := os.Remove(statePath); err != nil && !os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Warning: Failed to remove state file: %v\n", err)
	}

	// Remove workspace directory
	if err := os.RemoveAll(ws.Root); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to remove ephemeral workspace: %v\n", err)
	}
}
