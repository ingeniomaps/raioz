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

// prepareDockerResources handles validation of images, network, and volumes
func (uc *UseCase) prepareDockerResources(ctx context.Context, deps *config.Deps, ws *interfaces.Workspace) error {
	// Validate ports before starting
	// Use workspace name (not project name) because Docker Compose uses workspace name as project prefix
	baseDir := uc.deps.Workspace.GetBaseDirFromWorkspace(ws)
	workspaceNameForPorts := deps.GetWorkspaceName()
	conflicts, err := uc.deps.DockerRunner.ValidatePorts(deps, baseDir, workspaceNameForPorts)
	if err != nil {
		return errors.New(
			errors.ErrCodePortConflict,
			i18n.T("error.port_validate_failed"),
		).WithSuggestion(
			i18n.T("error.port_validate_suggestion"),
		).WithError(err)
	}
	if len(conflicts) > 0 {
		return errors.New(
			errors.ErrCodePortConflict,
			i18n.T("error.port_conflicts"),
		).WithSuggestion(
			i18n.T("error.port_conflicts_suggestion"),
		).WithContext("conflicts", uc.deps.DockerRunner.FormatPortConflicts(conflicts))
	}

	// Validate and pull Docker images before generating compose
	output.PrintProgress(i18n.T("up.verifying_images"))
	logging.DebugWithContext(ctx, "Validating and pulling Docker images")
	if err := uc.deps.DockerRunner.ValidateAllImages(deps); err != nil {
		logging.ErrorWithContext(ctx, "Failed to validate or pull Docker images", "error", err.Error())
		output.PrintProgressError(i18n.T("up.images_verify_error"))
		return errors.New(
			errors.ErrCodeImagePullFailed,
			i18n.T("error.image_pull_failed"),
		).WithSuggestion(
			i18n.T("error.image_pull_suggestion"),
		).WithError(err)
	}
	output.PrintProgressDone(i18n.T("up.images_verified"))

	// Ensure Docker network exists before generating compose
	networkName := deps.Network.GetName()
	networkSubnet := deps.Network.GetSubnet()

	// Determine if we should ask for confirmation
	// YAML projects (2.0) auto-generate network names — never prompt
	// Legacy projects: ask confirmation if network is configured as simple string
	askConfirmation := deps.SchemaVersion != "2.0" && (!deps.Network.IsObject || networkSubnet == "")

	output.PrintProgress(i18n.T("up.ensuring_network", networkName))
	if networkSubnet != "" {
		output.PrintInfo(i18n.T("up.network_subnet", networkSubnet))
	}
	logging.DebugWithContext(ctx, "Ensuring Docker network",
		"network", networkName, "subnet", networkSubnet, "askConfirmation", askConfirmation)

	err = uc.deps.DockerRunner.EnsureNetworkWithConfigAndContext(
		ctx, networkName, networkSubnet, askConfirmation,
	)
	if err != nil {
		logging.ErrorWithContext(ctx, "Failed to ensure Docker network",
			"network", networkName, "error", err.Error())
		output.PrintProgressError(i18n.T("up.network_ensure_error", networkName))
		return errors.New(
			errors.ErrCodeNetworkError,
			i18n.T("error.network_ensure_failed"),
		).WithSuggestion(
			i18n.T("error.network_ensure_suggestion"),
		).WithContext("network", networkName).WithError(err)
	}
	logging.DebugWithContext(ctx, "Docker network ready", "network", networkName)
	output.PrintProgressDone(i18n.T("up.network_ready", networkName))

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
			return errors.New(errors.ErrCodeVolumeError, i18n.T("error.volume_extract_infra")).WithError(err)
		}
		for _, volName := range named {
			n, err := uc.deps.DockerRunner.NormalizeVolumeName(workspaceName, volName)
			if err != nil {
				return errors.New(errors.ErrCodeVolumeError, i18n.T("error.volume_normalize", volName)).WithError(err)
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
			return errors.New(errors.ErrCodeVolumeError, i18n.T("error.volume_extract_services")).WithError(err)
		}
		for _, volName := range named {
			n, err := uc.deps.DockerRunner.NormalizeVolumeName(deps.Project.Name, volName)
			if err != nil {
				return errors.New(
					errors.ErrCodeVolumeError,
					i18n.T("error.volume_normalize_service", volName),
				).WithError(err)
			}
			if !seenNormalized[n] {
				seenNormalized[n] = true
				normalizedVolumes = append(normalizedVolumes, n)
			}
		}
	}
	for i, volName := range normalizedVolumes {
		output.PrintProgressStep(i+1, len(normalizedVolumes), i18n.T("up.ensuring_volume", volName))
		if err := uc.deps.DockerRunner.EnsureVolumeWithContext(ctx, volName); err != nil {
			output.PrintProgressError(i18n.T("up.volume_ensure_error", volName))
			return errors.New(
				errors.ErrCodeVolumeError,
				i18n.T("error.volume_ensure_failed", volName),
			).WithSuggestion(
				i18n.T("error.volume_ensure_suggestion"),
			).WithContext("volume", volName).WithError(err)
		}
	}
	if len(normalizedVolumes) > 0 {
		output.PrintProgressDone(i18n.T("up.volumes_ready", len(normalizedVolumes)))
	}

	return nil
}
