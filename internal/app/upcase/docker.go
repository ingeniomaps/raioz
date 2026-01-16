package upcase

import (
	"context"
	"fmt"

	"raioz/internal/config"
	"raioz/internal/domain/interfaces"
	"raioz/internal/docker"
	"raioz/internal/errors"
	"raioz/internal/logging"
	"raioz/internal/output"
	"raioz/internal/workspace"
)

// prepareDockerResources handles validation of images, network, and volumes
func (uc *UseCase) prepareDockerResources(ctx context.Context, deps *config.Deps, ws *interfaces.Workspace) error {
	// Convert interfaces.Workspace to concrete workspace.Workspace for operations that need it
	wsConcrete := (*workspace.Workspace)(ws)

	// Validate ports before starting
	baseDir := workspace.GetBaseDirFromWorkspace(wsConcrete)
	conflicts, err := docker.ValidatePorts(deps, baseDir, deps.Project.Name)
	if err != nil {
		return errors.New(errors.ErrCodePortConflict, "Failed to validate port configuration").WithSuggestion("Check that port validation can access workspace directories. " + "Verify file permissions and workspace structure.").WithError(err)
	}
	if len(conflicts) > 0 {
		return errors.New(errors.ErrCodePortConflict, "Port conflicts detected").WithSuggestion("Resolve port conflicts by changing port mappings in your configuration. " + "Each service must use unique ports.").WithContext("conflicts", docker.FormatPortConflicts(conflicts))
	}

	// Validate and pull Docker images before generating compose
	output.PrintProgress("Verifying and pulling Docker images")
	logging.DebugWithContext(ctx, "Validating and pulling Docker images")
	if err := docker.ValidateAllImages(deps); err != nil {
		logging.ErrorWithContext(ctx, "Failed to validate or pull Docker images", "error", err.Error())
		output.PrintProgressError("Failed to verify or pull Docker images")
		return errors.New(errors.ErrCodeImagePullFailed, "Failed to validate or pull Docker images").WithSuggestion("Check network connectivity and Docker daemon status. " + "Verify that image names and tags are correct. " + "Ensure you have permission to pull images. " + "Try running 'docker pull <image>:<tag>' manually to test.").WithError(err)
	}
	output.PrintProgressDone("All Docker images verified and ready")

	// Ensure Docker network exists before generating compose
	output.PrintProgress(fmt.Sprintf("Ensuring Docker network '%s'...", deps.Project.Network))
	logging.DebugWithContext(ctx, "Ensuring Docker network", "network", deps.Project.Network)
	if err := docker.EnsureNetworkWithContext(ctx, deps.Project.Network); err != nil {
		logging.ErrorWithContext(ctx, "Failed to ensure Docker network", "network", deps.Project.Network, "error", err.Error())
		output.PrintProgressError(fmt.Sprintf("Failed to ensure Docker network '%s'", deps.Project.Network))
		return errors.New(errors.ErrCodeNetworkError, "Failed to ensure Docker network").WithSuggestion("Check Docker daemon status and permissions. " + "Ensure you have permission to create Docker networks. " + "Try running 'docker network ls' to verify Docker is working.").WithContext("network", deps.Project.Network).WithError(err)
	}
	logging.DebugWithContext(ctx, "Docker network ready", "network", deps.Project.Network)
	output.PrintProgressDone(fmt.Sprintf("Docker network '%s' ready", deps.Project.Network))

	// Collect named volumes to show informative messages
	var allVolumes []string
	for _, svc := range deps.Services {
		// Skip if docker is nil (host execution - no docker volumes)
		if svc.Docker == nil {
			continue
		}
		allVolumes = append(allVolumes, svc.Docker.Volumes...)
	}
	for _, infra := range deps.Infra {
		allVolumes = append(allVolumes, infra.Volumes...)
	}
	namedVolumes, err := docker.ExtractNamedVolumes(allVolumes)
	if err != nil {
		return errors.New(errors.ErrCodeVolumeError, "Failed to extract named volumes from configuration").WithSuggestion("Check your volume configuration format. " + "Named volumes should follow the format 'volume_name:/path/in/container'.").WithError(err)
	}
	for i, volName := range namedVolumes {
		output.PrintProgressStep(i+1, len(namedVolumes), fmt.Sprintf("Ensuring Docker volume '%s'", volName))
		if err := docker.EnsureVolumeWithContext(ctx, volName); err != nil {
			output.PrintProgressError(fmt.Sprintf("Failed to ensure Docker volume '%s'", volName))
			return errors.New(errors.ErrCodeVolumeError, fmt.Sprintf("Failed to ensure Docker volume '%s'", volName)).WithSuggestion("Check Docker daemon status and permissions. " + "Ensure you have permission to create Docker volumes. " + "Try running 'docker volume ls' to verify Docker is working.").WithContext("volume", volName).WithError(err)
		}
	}
	if len(namedVolumes) > 0 {
		output.PrintProgressDone(fmt.Sprintf("All %d Docker volume(s) ready", len(namedVolumes)))
	}

	return nil
}
