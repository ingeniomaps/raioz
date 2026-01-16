package upcase

import (
	"context"

	"raioz/internal/config"
	"raioz/internal/domain/interfaces"
	"raioz/internal/errors"
	"raioz/internal/logging"
	"raioz/internal/output"
)

// bootstrap handles context, logging, panic recovery, and initial config/workspace loading
func (uc *UseCase) bootstrap(ctx context.Context, configPath string) (context.Context, *config.Deps, *interfaces.Workspace, error) {
	// Ensure context
	if ctx == nil {
		ctx = context.Background()
	}
	// Add request ID and operation context for logging correlation
	ctx = logging.WithRequestID(ctx)
	ctx = logging.WithOperation(ctx, "raioz up")

	// Load configuration first (needed for workspace resolution)
	deps, warnings, err := uc.deps.ConfigLoader.LoadDeps(configPath)
	if err != nil {
		return nil, nil, nil, errors.New(
			errors.ErrCodeInvalidConfig,
			"Failed to load configuration from "+configPath,
		).WithSuggestion(
			"Ensure .raioz.json exists and is valid JSON. Use --config flag to specify a different path.",
		).WithContext(
			"config_path", configPath,
		).WithError(err)
	}

	// Show deprecation warnings if any
	for _, warning := range warnings {
		output.PrintWarning(warning)
	}

	// Resolve workspace first (we need project name from deps)
	ws, err := uc.deps.Workspace.Resolve(deps.Project.Name)
	if err != nil {
		return nil, nil, nil, errors.New(
			errors.ErrCodeWorkspaceError, "Failed to resolve workspace",
		).WithSuggestion(
			"Ensure you have write permissions for workspace directories. Check README.md for workspace locations.",
		).WithContext("project", deps.Project.Name).WithError(err)
	}

	// Note: raioz.root.json is used for metadata tracking (service origins, etc.)
	// but .raioz.json is always the source of truth for actual configuration values.
	// We load .raioz.json above and use it throughout the process.
	// raioz.root.json will be updated at the end with the current .raioz.json values.

	return ctx, deps, ws, nil
}
