package upcase

import (
	"context"
	"fmt"

	"raioz/internal/config"
	"raioz/internal/domain/interfaces"
	"raioz/internal/errors"
	"raioz/internal/logging"
	"raioz/internal/output"
)

// prepareDockerResources handles validation of images, network, and volumes
func (uc *UseCase) prepareDockerResources(ctx context.Context, deps *config.Deps, ws *interfaces.Workspace) error {
	// Validate ports before starting
	baseDir := uc.deps.Workspace.GetBaseDirFromWorkspace(ws)
	conflicts, err := uc.deps.DockerRunner.ValidatePorts(deps, baseDir, deps.Project.Name)
	if err != nil {
		return errors.New(errors.ErrCodePortConflict, "Failed to validate port configuration").WithSuggestion("Check that port validation can access workspace directories. " + "Verify file permissions and workspace structure.").WithError(err)
	}
	if len(conflicts) > 0 {
		return errors.New(errors.ErrCodePortConflict, "Port conflicts detected").WithSuggestion("Resolve port conflicts by changing port mappings in your configuration. " + "Each service must use unique ports.").WithContext("conflicts", uc.deps.DockerRunner.FormatPortConflicts(conflicts))
	}

	// Validate and pull Docker images before generating compose
	output.PrintProgress("Verifying and pulling Docker images")
	logging.DebugWithContext(ctx, "Validating and pulling Docker images")
	if err := uc.deps.DockerRunner.ValidateAllImages(deps); err != nil {
		logging.ErrorWithContext(ctx, "Failed to validate or pull Docker images", "error", err.Error())
		output.PrintProgressError("Failed to verify or pull Docker images")
		return errors.New(errors.ErrCodeImagePullFailed, "Failed to validate or pull Docker images").WithSuggestion("Check network connectivity and Docker daemon status. " + "Verify that image names and tags are correct. " + "Ensure you have permission to pull images. " + "Try running 'docker pull <image>:<tag>' manually to test.").WithError(err)
	}
	output.PrintProgressDone("All Docker images verified and ready")

	// Ensure Docker network exists before generating compose
	networkName := deps.Network.GetName()
	networkSubnet := deps.Network.GetSubnet()

	// Determine if we should ask for confirmation
	// Ask confirmation if network is configured as simple string (backward compatible behavior)
	askConfirmation := !deps.Network.IsObject || networkSubnet == ""

	output.PrintProgress(fmt.Sprintf("Ensuring Docker network '%s'...", networkName))
	if networkSubnet != "" {
		output.PrintInfo(fmt.Sprintf("   Subnet: %s", networkSubnet))
	}
	logging.DebugWithContext(ctx, "Ensuring Docker network", "network", networkName, "subnet", networkSubnet, "askConfirmation", askConfirmation)

	if err := uc.deps.DockerRunner.EnsureNetworkWithConfigAndContext(ctx, networkName, networkSubnet, askConfirmation); err != nil {
		logging.ErrorWithContext(ctx, "Failed to ensure Docker network", "network", networkName, "error", err.Error())
		output.PrintProgressError(fmt.Sprintf("Failed to ensure Docker network '%s'", networkName))
		return errors.New(errors.ErrCodeNetworkError, "Failed to ensure Docker network").WithSuggestion("Check Docker daemon status and permissions. " + "Ensure you have permission to create Docker networks. " + "Try running 'docker network ls' to verify Docker is working.").WithContext("network", networkName).WithError(err)
	}
	logging.DebugWithContext(ctx, "Docker network ready", "network", networkName)
	output.PrintProgressDone(fmt.Sprintf("Docker network '%s' ready", networkName))

	// Infra volumes: workspace prefix (shared). Service volumes: project prefix.
	workspaceName := deps.GetWorkspaceName()
	seenNormalized := make(map[string]bool)
	var normalizedVolumes []string
	for _, entry := range deps.Infra {
		if entry.Inline == nil {
			continue
		}
		named, err := uc.deps.DockerRunner.ExtractNamedVolumes(entry.Inline.Volumes)
		if err != nil {
			return errors.New(errors.ErrCodeVolumeError, "Failed to extract named volumes from infra").WithError(err)
		}
		for _, volName := range named {
			n, err := uc.deps.DockerRunner.NormalizeVolumeName(workspaceName, volName)
			if err != nil {
				return errors.New(errors.ErrCodeVolumeError, fmt.Sprintf("Failed to normalize infra volume '%s'", volName)).WithError(err)
			}
			if !seenNormalized[n] {
				seenNormalized[n] = true
				normalizedVolumes = append(normalizedVolumes, n)
			}
		}
	}
	for _, svc := range deps.Services {
		if svc.Docker == nil {
			continue
		}
		named, err := uc.deps.DockerRunner.ExtractNamedVolumes(svc.Docker.Volumes)
		if err != nil {
			return errors.New(errors.ErrCodeVolumeError, "Failed to extract named volumes from services").WithError(err)
		}
		for _, volName := range named {
			n, err := uc.deps.DockerRunner.NormalizeVolumeName(deps.Project.Name, volName)
			if err != nil {
				return errors.New(errors.ErrCodeVolumeError, fmt.Sprintf("Failed to normalize service volume '%s'", volName)).WithError(err)
			}
			if !seenNormalized[n] {
				seenNormalized[n] = true
				normalizedVolumes = append(normalizedVolumes, n)
			}
		}
	}
	for i, volName := range normalizedVolumes {
		output.PrintProgressStep(i+1, len(normalizedVolumes), fmt.Sprintf("Ensuring Docker volume '%s'", volName))
		if err := uc.deps.DockerRunner.EnsureVolumeWithContext(ctx, volName); err != nil {
			output.PrintProgressError(fmt.Sprintf("Failed to ensure Docker volume '%s'", volName))
			return errors.New(errors.ErrCodeVolumeError, fmt.Sprintf("Failed to ensure Docker volume '%s'", volName)).WithSuggestion("Check Docker daemon status and permissions. " + "Ensure you have permission to create Docker volumes. " + "Try running 'docker volume ls' to verify Docker is working.").WithContext("volume", volName).WithError(err)
		}
	}
	if len(normalizedVolumes) > 0 {
		output.PrintProgressDone(fmt.Sprintf("All %d Docker volume(s) ready", len(normalizedVolumes)))
	}

	return nil
}
