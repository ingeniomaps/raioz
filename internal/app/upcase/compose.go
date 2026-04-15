package upcase

import (
	"context"
	"time"

	"raioz/internal/config"
	"raioz/internal/docker"
	"raioz/internal/domain/interfaces"
	"raioz/internal/errors"
	"raioz/internal/i18n"
	"raioz/internal/logging"
	"raioz/internal/output"
)

// processCompose handles generation of docker-compose and docker.Up
func (uc *UseCase) processCompose(
	ctx context.Context,
	deps *config.Deps,
	ws *interfaces.Workspace,
	projectDir string,
) (string, []string, []string, error) {
	// Collect service names (only Docker services, host services are handled separately)
	var serviceNames []string
	var disabledServices []string
	for name, svc := range deps.Services {
		// Check if service is disabled (already filtered by FilterByFeatureFlags, but check enabled field)
		if svc.Enabled != nil && !*svc.Enabled {
			disabledServices = append(disabledServices, name)
			output.PrintInfo(i18n.T("up.service_disabled_skipping", name))
			continue
		}
		// Skip host services (source.command exists means host execution)
		if svc.Source.Command != "" {
			continue
		}
		// Skip services with custom commands (no docker, no source.command, but has commands)
		if svc.Docker == nil && svc.Commands != nil {
			continue
		}
		if svc.Source.Kind == "image" {
			output.PrintServiceUsingImage(name)
		}
		serviceNames = append(serviceNames, name)
	}
	if len(disabledServices) > 0 {
		output.PrintInfo(i18n.T("up.skipped_disabled_services", len(disabledServices), disabledServices))
	}

	// Process infra (inline from config; external YAML names come after GenerateCompose)
	var infraNames []string
	for name := range deps.Infra {
		infraNames = append(infraNames, name)
	}

	// If no services, no inline infra, and no external infra file, return early
	if len(serviceNames) == 0 && len(infraNames) == 0 {
		return "", []string{}, []string{}, nil
	}

	// Generate Docker Compose configuration (may merge external infra YAML)
	output.PrintProgress(i18n.T("up.generating_compose"))
	composeStartTime := time.Now()
	composePath, externalInfraNames, err := uc.deps.DockerRunner.GenerateCompose(deps, ws, projectDir)
	if err != nil {
		logging.ErrorWithContext(ctx, "Failed to generate Docker Compose configuration",
			"duration_ms", time.Since(composeStartTime).Milliseconds(), "error", err.Error())
		output.PrintProgressError(i18n.T("up.compose_generate_error"))
		return "", nil, nil, errors.New(
			errors.ErrCodeWorkspaceError,
			i18n.T("error.compose_generate_failed"),
		).WithSuggestion(i18n.T("error.compose_generate_suggestion")).WithError(err)
	}
	logging.InfoWithContext(ctx, "Docker Compose configuration generated",
		"compose_path", composePath, "duration_ms", time.Since(composeStartTime).Milliseconds())
	output.PrintProgressDone(i18n.T("up.compose_generated"))

	// Include services from external infra YAML in infra list (start them first)
	infraNames = append(infraNames, externalInfraNames...)
	logging.InfoWithContext(ctx, "Generating Docker Compose configuration",
		"services_count", len(serviceNames), "infra_count", len(infraNames))

	// Deploy in order: infra first, then services
	// This ensures infrastructure (databases, etc.) is available and healthy before services start
	upStartTime := time.Now()

	// Scope all compose operations with an explicit project name so
	// --remove-orphans only affects THIS project's containers, never
	// containers from other projects that share the directory basename.
	scopedCtx := docker.WithComposeProjectName(ctx, "raioz-"+deps.Project.Name)

	// Step 1: Deploy infra first (if any)
	if len(infraNames) > 0 {
		output.PrintProgress(i18n.T("up.starting_infra", len(infraNames)))
		logging.InfoWithContext(ctx, "Starting infrastructure services",
			"compose_path", composePath, "infra_count", len(infraNames), "infra_names", infraNames)
		if err := uc.deps.DockerRunner.UpServicesWithContext(scopedCtx, composePath, infraNames); err != nil {
			logging.ErrorWithContext(ctx, "Failed to start infrastructure services",
				"compose_path", composePath, "duration_ms", time.Since(upStartTime).Milliseconds(),
				"error", err.Error())
			output.PrintProgressError(i18n.T("up.infra_start_error"))
			return "", nil, nil, errors.New(
				errors.ErrCodeDockerNotRunning,
				i18n.T("error.infra_start_failed"),
			).WithSuggestion(
				i18n.T("error.infra_start_suggestion"),
			).WithContext("compose_file", composePath).WithError(err)
		}
		logging.InfoWithContext(ctx, "Infrastructure services started successfully",
			"compose_path", composePath, "duration_ms", time.Since(upStartTime).Milliseconds())
		// Show individual infra services as started
		for _, name := range infraNames {
			output.PrintInfraStarted(name)
		}

		// Wait for infra to be healthy before proceeding with services
		// This ensures databases and other infrastructure are ready before services try to connect
		output.PrintProgress(i18n.T("up.waiting_infra_healthy"))
		logging.InfoWithContext(ctx, "Waiting for infrastructure services to become healthy",
			"infra_names", infraNames)
		err := uc.deps.DockerRunner.WaitForServicesHealthy(
			ctx, composePath, []string{}, infraNames, deps.Project.Name,
		)
		if err != nil {
			logging.WarnWithContext(ctx, "Infrastructure services may not be fully healthy yet", "error", err.Error())
			output.PrintWarning(i18n.T("up.infra_not_healthy_warning"))
			// Continue anyway - user may want to proceed even if health checks fail
		} else {
			output.PrintProgressDone(i18n.T("up.infra_healthy"))
		}
	}

	// Step 2: Deploy services (if any)
	// Services can now safely connect to infrastructure since it's healthy
	if len(serviceNames) > 0 {
		output.PrintProgress(i18n.T("up.starting_services", len(serviceNames)))
		logging.InfoWithContext(ctx, "Starting application services",
			"compose_path", composePath, "services_count", len(serviceNames),
			"service_names", serviceNames)
		servicesStartTime := time.Now()
		if err := uc.deps.DockerRunner.UpServicesWithContext(scopedCtx, composePath, serviceNames); err != nil {
			logging.ErrorWithContext(ctx, "Failed to start application services",
				"compose_path", composePath,
				"duration_ms", time.Since(servicesStartTime).Milliseconds(),
				"error", err.Error())
			output.PrintProgressError(i18n.T("up.services_start_error"))
			return "", nil, nil, errors.New(
				errors.ErrCodeDockerNotRunning,
				i18n.T("error.services_start_failed"),
			).WithSuggestion(
				i18n.T("error.services_start_suggestion"),
			).WithContext("compose_file", composePath).WithError(err)
		}
		logging.InfoWithContext(ctx, "Application services started successfully",
			"compose_path", composePath,
			"duration_ms", time.Since(servicesStartTime).Milliseconds())
		output.PrintProgressDone(i18n.T("up.services_started", len(serviceNames)))
	}

	logging.InfoWithContext(ctx, "All Docker Compose services started successfully",
		"compose_path", composePath, "total_duration_ms", time.Since(upStartTime).Milliseconds())

	return composePath, serviceNames, infraNames, nil
}
