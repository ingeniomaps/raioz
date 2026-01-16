package upcase

import (
	"context"

	"raioz/internal/config"
	"raioz/internal/domain/interfaces"
	"raioz/internal/docker"
	"raioz/internal/errors"
	"raioz/internal/logging"
	"raioz/internal/output"
	"raioz/internal/validate"
	"raioz/internal/workspace"
)

// validate handles validate.All, workspace permissions, port validation, and dependency conflicts
func (uc *UseCase) validate(ctx context.Context, deps *config.Deps, ws *interfaces.Workspace, dryRun bool) error {
	// Step 1: Preflight checks (Docker, Git, disk space, network) - as documented
	if err := validate.PreflightCheckWithContext(ctx); err != nil {
		return errors.New(
			errors.ErrCodeDockerNotInstalled,
			"Preflight checks failed",
		).WithSuggestion(
			"Ensure Docker is installed and running, Git is installed, and you have sufficient disk space. "+
				"Check the error details above for specific issues.",
		).WithError(err)
	}

	// Step 2: Perform comprehensive configuration validation
	if err := uc.deps.Validator.All(deps); err != nil {
		return err
	}

	// Convert interfaces.Workspace to concrete workspace.Workspace for operations that need it
	wsConcrete := (*workspace.Workspace)(ws)

	// Perform migration of legacy services if needed
	if err := workspace.MigrateLegacyServices(wsConcrete, deps); err != nil {
		// Log but don't fail - migration is best-effort
		output.PrintWarning("Migration warning: " + err.Error())
	}

	// Detect and warn about shared volumes
	serviceVolumes, err := docker.BuildServiceVolumesMap(deps)
	if err != nil {
		// Log error but don't fail - volume detection is informational
		logging.Warn("Failed to build service volumes map", "error", err)
	} else {
		sharedVolumes := docker.DetectSharedVolumes(serviceVolumes)
		if len(sharedVolumes) > 0 {
			warningMsg := docker.FormatSharedVolumesWarning(sharedVolumes)
			output.PrintWarning(warningMsg)
		}
	}

	// Check for dependency conflicts first (before checking permissions)
	shouldContinue, _, err := uc.handleDependencyConflicts(deps, wsConcrete, dryRun)
	if err != nil {
		return errors.New(errors.ErrCodeDependencyCycle, "Failed to handle dependency conflicts").WithSuggestion("Check your configuration for conflicting service definitions. " + "Resolve conflicts manually or use dependency assist.").WithError(err)
	}
	if !shouldContinue {
		if dryRun {
			return nil
			// Dry-run mode: just show conflicts, don't fail
		}
		return errors.New(errors.ErrCodeDependencyCycle, "Aborted due to dependency conflicts").WithSuggestion("Resolve the conflicts shown above and try again.")
	}

	// Check for missing dependencies
	shouldContinue, _, err = uc.handleDependencyAssist(deps, wsConcrete, dryRun)
	if err != nil {
		return errors.New(errors.ErrCodeInvalidConfig, "Failed to handle dependency assist").WithSuggestion("Check that dependency definitions are accessible. " + "Verify network connectivity and repository access.").WithError(err)
	}
	if !shouldContinue {
		if dryRun {
			return nil
			// Dry-run mode: just show missing dependencies, don't fail
		}
		return errors.New(errors.ErrCodeInvalidConfig, "Aborted by user during dependency assist").WithSuggestion("Resolve missing dependencies and try again.")
	}

	// Check workspace permissions
	if err := validate.CheckWorkspacePermissions(ws.Root); err != nil {
		return err
	}

	// Check if workspace was created (implicit check)
	output.PrintWorkspaceCreated()

	return nil
}
