package upcase

import (
	"context"

	"raioz/internal/config"
	"raioz/internal/domain/interfaces"
	"raioz/internal/errors"
	"raioz/internal/i18n"
	"raioz/internal/logging"
	"raioz/internal/naming"
	"raioz/internal/output"
)

// bootstrapResult holds the results of bootstrap phase
type bootstrapResult struct {
	ctx              context.Context
	deps             *config.Deps
	ws               *interfaces.Workspace
	appliedOverrides []string
}

// bootstrap handles context, logging, panic recovery, and initial config/workspace loading
func (uc *UseCase) bootstrap(ctx context.Context, configPath string) (*bootstrapResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx = logging.WithRequestID(ctx)
	ctx = logging.WithOperation(ctx, "raioz up")

	// Load configuration
	deps, warnings, err := uc.deps.ConfigLoader.LoadDeps(configPath)
	if err != nil {
		return nil, errors.New(
			errors.ErrCodeInvalidConfig,
			i18n.T("error.config_load_from", configPath),
		).WithSuggestion(
			i18n.T("error.config_load_from_suggestion"),
		).WithContext(
			"config_path", configPath,
		).WithError(err)
	}

	for _, warning := range warnings {
		output.PrintWarning(warning)
	}

	// Set naming prefix from workspace (if configured)
	naming.SetPrefix(deps.Workspace)

	// Apply overrides (registered via 'raioz override') before any processing
	deps, appliedOverrides, err := config.ApplyOverrides(deps)
	if err != nil {
		return nil, errors.New(
			errors.ErrCodeInvalidConfig,
			i18n.T("error.override_apply"),
		).WithSuggestion(
			i18n.T("error.override_apply_suggestion"),
		).WithError(err)
	}
	if len(appliedOverrides) > 0 {
		output.PrintInfo(i18n.T("up.overrides_applied", len(appliedOverrides), appliedOverrides))
	}

	// Resolve workspace
	workspaceName := deps.GetWorkspaceName()
	ws, err := uc.deps.Workspace.Resolve(workspaceName)
	if err != nil {
		return nil, errors.New(
			errors.ErrCodeWorkspaceError, i18n.T("error.workspace_resolve"),
		).WithSuggestion(
			i18n.T("error.workspace_resolve_permissions_suggestion"),
		).WithContext("project", deps.Project.Name).WithError(err)
	}

	return &bootstrapResult{
		ctx:              ctx,
		deps:             deps,
		ws:               ws,
		appliedOverrides: appliedOverrides,
	}, nil
}
