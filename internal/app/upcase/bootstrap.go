package upcase

import (
	"context"

	"raioz/internal/config"
	"raioz/internal/domain/interfaces"
	"raioz/internal/errors"
	"raioz/internal/i18n"
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
			i18n.T("error.config_load_from", configPath),
		).WithSuggestion(
			i18n.T("error.config_load_from_suggestion"),
		).WithContext(
			"config_path", configPath,
		).WithError(err)
	}

	// Show deprecation warnings if any
	for _, warning := range warnings {
		output.PrintWarning(warning)
	}

	// Resolve workspace first (use workspace name if specified, otherwise use project name)
	workspaceName := deps.GetWorkspaceName()
	ws, err := uc.deps.Workspace.Resolve(workspaceName)
	if err != nil {
		return nil, nil, nil, errors.New(
			errors.ErrCodeWorkspaceError, i18n.T("error.workspace_resolve"),
		).WithSuggestion(
			i18n.T("error.workspace_resolve_permissions_suggestion"),
		).WithContext("project", deps.Project.Name).WithError(err)
	}

	// Note: raioz.root.json is used for metadata tracking (service origins, etc.)
	// but .raioz.json is always the source of truth for actual configuration values.
	// We load .raioz.json above and use it throughout the process.
	// raioz.root.json will be updated at the end with the current .raioz.json values.

	return ctx, deps, ws, nil
}
