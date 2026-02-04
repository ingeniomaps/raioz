package upcase

import (
	"context"
	"fmt"
	"time"

	"raioz/internal/config"
	"raioz/internal/domain/interfaces"
	"raioz/internal/docker"
	"raioz/internal/errors"
	"raioz/internal/logging"
	"raioz/internal/output"
	"raioz/internal/workspace"
)

// processCompose handles generation of docker-compose and docker.Up
func (uc *UseCase) processCompose(ctx context.Context, deps *config.Deps, ws *interfaces.Workspace, projectDir string) (string, []string, []string, error) {
	// Convert interfaces.Workspace to concrete workspace.Workspace for operations that need it
	wsConcrete := (*workspace.Workspace)(ws)

	// Collect service names (only Docker services, host services are handled separately)
	var serviceNames []string
	var disabledServices []string
	for name, svc := range deps.Services {
		// Check if service is disabled (already filtered by FilterByFeatureFlags, but check enabled field)
		if svc.Enabled != nil && !*svc.Enabled {
			disabledServices = append(disabledServices, name)
			output.PrintInfo(fmt.Sprintf("Service %s is disabled, skipping", name))
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
		output.PrintInfo(fmt.Sprintf("Skipped %d disabled service(s): %v", len(disabledServices), disabledServices))
	}

	// Process infra
	var infraNames []string
	for name := range deps.Infra {
		infraNames = append(infraNames, name)
	}

	// If no services and no infra, return early
	if len(serviceNames) == 0 && len(infraNames) == 0 {
		return "", []string{}, []string{}, nil
	}

	// Generate Docker Compose configuration
	output.PrintProgress("Generating Docker Compose configuration")
	logging.InfoWithContext(ctx, "Generating Docker Compose configuration", "services_count", len(serviceNames), "infra_count", len(infraNames))
	composeStartTime := time.Now()
	composePath, err := docker.GenerateCompose(deps, wsConcrete, projectDir)
	if err != nil {
		logging.ErrorWithContext(ctx, "Failed to generate Docker Compose configuration", "duration_ms", time.Since(composeStartTime).Milliseconds(), "error", err.Error())
		output.PrintProgressError("Failed to generate Docker Compose configuration")
		return "", nil, nil, errors.New(errors.ErrCodeWorkspaceError, "Failed to generate Docker Compose configuration").WithSuggestion("Check your service configuration for errors. " + "Verify that all required fields are present and valid. " + "Review the error details above for specific issues.").WithError(err)
	}
	logging.InfoWithContext(ctx, "Docker Compose configuration generated", "compose_path", composePath, "duration_ms", time.Since(composeStartTime).Milliseconds())
	output.PrintProgressDone("Docker Compose configuration generated")

	// Deploy in order: infra first, then services
	// This ensures infrastructure (databases, etc.) is available and healthy before services start
	upStartTime := time.Now()

	// Step 1: Deploy infra first (if any)
	if len(infraNames) > 0 {
		output.PrintProgress(fmt.Sprintf("Starting infrastructure services (%d)...", len(infraNames)))
		logging.InfoWithContext(ctx, "Starting infrastructure services", "compose_path", composePath, "infra_count", len(infraNames), "infra_names", infraNames)
		if err := docker.UpServicesWithContext(ctx, composePath, infraNames); err != nil {
			logging.ErrorWithContext(ctx, "Failed to start infrastructure services", "compose_path", composePath, "duration_ms", time.Since(upStartTime).Milliseconds(), "error", err.Error())
			output.PrintProgressError("Failed to start infrastructure services")
			return "", nil, nil, errors.New(errors.ErrCodeDockerNotRunning, "Failed to start infrastructure services").WithSuggestion("Check Docker daemon status with 'docker ps'. " + "Verify that Docker Compose is installed and working. " + "Check logs with 'docker compose logs' for more details.").WithContext("compose_file", composePath).WithError(err)
		}
		logging.InfoWithContext(ctx, "Infrastructure services started successfully", "compose_path", composePath, "duration_ms", time.Since(upStartTime).Milliseconds())
		// Show individual infra services as started
		for _, name := range infraNames {
			output.PrintInfraStarted(name)
		}

		// Wait for infra to be healthy before proceeding with services
		// This ensures databases and other infrastructure are ready before services try to connect
		output.PrintProgress("Waiting for infrastructure to be healthy...")
		logging.InfoWithContext(ctx, "Waiting for infrastructure services to become healthy", "infra_names", infraNames)
		if err := docker.WaitForServicesHealthy(ctx, composePath, []string{}, infraNames, deps.Project.Name); err != nil {
			logging.WarnWithContext(ctx, "Infrastructure services may not be fully healthy yet", "error", err.Error())
			output.PrintWarning("Some infrastructure services may not be fully healthy yet, proceeding with services deployment anyway")
			// Continue anyway - user may want to proceed even if health checks fail
		} else {
			output.PrintProgressDone("Infrastructure is healthy and ready")
		}
	}

	// Step 2: Deploy services (if any)
	// Services can now safely connect to infrastructure since it's healthy
	if len(serviceNames) > 0 {
		output.PrintProgress(fmt.Sprintf("Starting application services (%d)...", len(serviceNames)))
		logging.InfoWithContext(ctx, "Starting application services", "compose_path", composePath, "services_count", len(serviceNames), "service_names", serviceNames)
		servicesStartTime := time.Now()
		if err := docker.UpServicesWithContext(ctx, composePath, serviceNames); err != nil {
			logging.ErrorWithContext(ctx, "Failed to start application services", "compose_path", composePath, "duration_ms", time.Since(servicesStartTime).Milliseconds(), "error", err.Error())
			output.PrintProgressError("Failed to start application services")
			return "", nil, nil, errors.New(errors.ErrCodeDockerNotRunning, "Failed to start application services").WithSuggestion("Check Docker daemon status with 'docker ps'. " + "Verify that Docker Compose is installed and working. " + "Check logs with 'docker compose logs' for more details.").WithContext("compose_file", composePath).WithError(err)
		}
		logging.InfoWithContext(ctx, "Application services started successfully", "compose_path", composePath, "duration_ms", time.Since(servicesStartTime).Milliseconds())
		output.PrintProgressDone(fmt.Sprintf("Application services started (%d)", len(serviceNames)))
	}

	logging.InfoWithContext(ctx, "All Docker Compose services started successfully", "compose_path", composePath, "total_duration_ms", time.Since(upStartTime).Milliseconds())

	return composePath, serviceNames, infraNames, nil
}
