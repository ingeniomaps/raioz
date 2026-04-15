package app

import (
	"context"

	"raioz/internal/errors"
	"raioz/internal/i18n"
	"raioz/internal/logging"
	"raioz/internal/output"
)

// HealthOptions holds options for the health use case
type HealthOptions struct {
	ConfigPath string
}

// HealthUseCase handles health checking for local projects
type HealthUseCase struct {
	deps *Dependencies
}

// NewHealthUseCase creates a new HealthUseCase
func NewHealthUseCase(deps *Dependencies) *HealthUseCase {
	return &HealthUseCase{deps: deps}
}

// Execute runs the health use case
func (uc *HealthUseCase) Execute(ctx context.Context, opts HealthOptions) error {
	ctx = logging.WithRequestID(ctx)
	ctx = logging.WithOperation(ctx, "raioz health")

	// Load configuration
	configDeps, warnings, err := uc.deps.ConfigLoader.LoadDeps(opts.ConfigPath)
	for _, w := range warnings {
		output.PrintWarning(w)
	}
	if err != nil {
		return errors.New(
			errors.ErrCodeInvalidConfig,
			i18n.T("error.config_load"),
		).WithSuggestion(
			i18n.T("error.config_load_suggestion"),
		).WithError(err)
	}

	// Get base dir for local project check
	baseDir, err := uc.deps.Workspace.GetBaseDir()
	if err != nil {
		return errors.New(
			errors.ErrCodeWorkspaceError,
			i18n.T("error.base_dir"),
		).WithError(err)
	}

	// Check if this is a local project
	isLocal, projectDir, err := IsLocalProject(opts.ConfigPath, baseDir)
	if err != nil {
		return errors.New(
			errors.ErrCodeWorkspaceError,
			i18n.T("error.local_check"),
		).WithError(err)
	}

	if !isLocal {
		output.PrintInfo(i18n.T("output.not_local_project"))
		return nil
	}

	// Determine mode
	mode := "dev"
	for _, svc := range configDeps.Services {
		if svc.Docker != nil && svc.Docker.Mode != "" {
			mode = svc.Docker.Mode
			break
		}
	}

	// Get health command
	healthCommand := GetLocalProjectCommand(configDeps, "health", mode)

	// Check health
	isHealthy, err := CheckLocalProjectHealth(ctx, projectDir, healthCommand)
	if err != nil {
		return errors.New(
			errors.ErrCodeWorkspaceError,
			i18n.T("error.health_check"),
		).WithError(err)
	}

	if isHealthy {
		output.PrintSuccess(i18n.T("output.project_healthy"))
		return nil
	}

	// Project is not healthy
	output.PrintWarning(i18n.T("output.project_not_healthy"))
	return nil
}
