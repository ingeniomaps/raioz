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
func (uc *UseCase) processCompose(ctx context.Context, deps *config.Deps, ws *interfaces.Workspace) (string, []string, []string, error) {
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
		output.PrintInfraStarted(name)
	}

	// If no services and no infra, return early
	if len(serviceNames) == 0 && len(infraNames) == 0 {
		return "", []string{}, []string{}, nil
	}

	// Generate Docker Compose configuration
	output.PrintProgress("Generating Docker Compose configuration")
	logging.InfoWithContext(ctx, "Generating Docker Compose configuration", "services_count", len(serviceNames), "infra_count", len(infraNames))
	composeStartTime := time.Now()
	composePath, err := docker.GenerateCompose(deps, wsConcrete)
	if err != nil {
		logging.ErrorWithContext(ctx, "Failed to generate Docker Compose configuration", "duration_ms", time.Since(composeStartTime).Milliseconds(), "error", err.Error())
		output.PrintProgressError("Failed to generate Docker Compose configuration")
		return "", nil, nil, errors.New(errors.ErrCodeWorkspaceError, "Failed to generate Docker Compose configuration").WithSuggestion("Check your service configuration for errors. " + "Verify that all required fields are present and valid. " + "Review the error details above for specific issues.").WithError(err)
	}
	logging.InfoWithContext(ctx, "Docker Compose configuration generated", "compose_path", composePath, "duration_ms", time.Since(composeStartTime).Milliseconds())
	output.PrintProgressDone("Docker Compose configuration generated")

	// Start Docker Compose services
	output.PrintProgress("Starting Docker Compose services")
	logging.InfoWithContext(ctx, "Starting Docker Compose services", "compose_path", composePath, "services_count", len(serviceNames), "infra_count", len(infraNames))
	upStartTime := time.Now()
	if err := uc.deps.DockerRunner.Up(composePath); err != nil {
		logging.ErrorWithContext(ctx, "Failed to start Docker Compose services", "compose_path", composePath, "duration_ms", time.Since(upStartTime).Milliseconds(), "error", err.Error())
		output.PrintProgressError("Failed to start Docker Compose services")
		return "", nil, nil, errors.New(errors.ErrCodeDockerNotRunning, "Failed to start Docker Compose services").WithSuggestion("Check Docker daemon status with 'docker ps'. " + "Verify that Docker Compose is installed and working. " + "Check logs with 'docker compose logs' for more details.").WithContext("compose_file", composePath).WithError(err)
	}
	logging.InfoWithContext(ctx, "Docker Compose services started successfully", "compose_path", composePath, "duration_ms", time.Since(upStartTime).Milliseconds())
	output.PrintProgressDone("Docker Compose services started successfully")

	return composePath, serviceNames, infraNames, nil
}
