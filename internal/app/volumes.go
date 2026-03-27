package app

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"raioz/internal/config"
	"raioz/internal/domain/interfaces"
	"raioz/internal/errors"
	"raioz/internal/i18n"
	"raioz/internal/logging"
	"raioz/internal/output"
)

// VolumesOptions contains options for the Volumes use case
type VolumesOptions struct {
	ConfigPath  string
	ProjectName string
	Force       bool // Skip confirmation prompt
}

// VolumesUseCase handles the "volumes" use case - removing project volumes
type VolumesUseCase struct {
	deps *Dependencies
}

// NewVolumesUseCase creates a new VolumesUseCase with injected dependencies
func NewVolumesUseCase(deps *Dependencies) *VolumesUseCase {
	return &VolumesUseCase{
		deps: deps,
	}
}

// Execute executes the volumes use case
func (uc *VolumesUseCase) Execute(ctx context.Context, opts VolumesOptions) error {
	// Add request ID and operation context for logging correlation
	ctx = logging.WithRequestID(ctx)
	ctx = logging.WithOperation(ctx, "raioz volumes")

	var ws *interfaces.Workspace
	var err error

	// Determine project name and workspace
	projectName := opts.ProjectName
	var workspaceName string
	if projectName == "" {
		logging.DebugWithContext(ctx, "Project name not provided, loading from config",
			"config_path", opts.ConfigPath,
		)
		deps, _, _ := uc.deps.ConfigLoader.LoadDeps(opts.ConfigPath)
		if deps != nil {
			projectName = deps.Project.Name
			workspaceName = deps.GetWorkspaceName()
			ctx = logging.WithProject(ctx, projectName)
		} else {
			logging.ErrorWithContext(ctx, "Could not determine project name")
			return errors.New(
				errors.ErrCodeInvalidConfig,
				i18n.T("error.no_project"),
			).WithSuggestion(
				i18n.T("error.no_project_suggestion"),
			)
		}
	} else {
		ctx = logging.WithProject(ctx, projectName)
		// If project name comes from CLI, load config to get workspace name
		deps, _, _ := uc.deps.ConfigLoader.LoadDeps(opts.ConfigPath)
		if deps != nil && deps.Project.Name == projectName {
			workspaceName = deps.GetWorkspaceName()
		} else {
			// Fallback: use project name as workspace (backward compatibility)
			workspaceName = projectName
		}
	}

	// Log operation start
	logging.LogOperationStart(ctx, "raioz volumes",
		"project", projectName,
	)

	// Resolve workspace using workspace name
	ws, err = uc.deps.Workspace.Resolve(workspaceName)
	if err != nil {
		logging.ErrorWithContext(ctx, "Failed to resolve workspace",
			"project", projectName,
			"error", err.Error(),
		)
		return errors.New(
			errors.ErrCodeWorkspaceError,
			i18n.T("error.workspace_resolve"),
		).WithSuggestion(
			i18n.T("error.workspace_resolve_suggestion"),
		).WithContext("project", projectName).WithError(err)
	}

	// Get workspace root using interface method
	wsRoot := uc.deps.Workspace.GetRoot(ws)
	logging.InfoWithContext(ctx, "Workspace resolved",
		"workspace", wsRoot,
	)

	// Try to load state to get volumes from state (same source as compose generation)
	var serviceVolumes []string
	var infraVolumes []string
	var stateDeps *config.Deps

	if uc.deps.StateManager.Exists(ws) {
		stateDeps, err = uc.deps.StateManager.Load(ws)
		if err == nil {
			for _, svc := range stateDeps.Services {
				if svc.Docker != nil {
					serviceVolumes = append(serviceVolumes, svc.Docker.Volumes...)
				}
			}
			for _, entry := range stateDeps.Infra {
				if entry.Inline != nil {
					infraVolumes = append(infraVolumes, entry.Inline.Volumes...)
				}
			}
		}
	}

	if len(serviceVolumes) == 0 && len(infraVolumes) == 0 {
		deps, _, _ := uc.deps.ConfigLoader.LoadDeps(opts.ConfigPath)
		if deps != nil {
			stateDeps = deps
			for _, svc := range deps.Services {
				if svc.Docker != nil {
					serviceVolumes = append(serviceVolumes, svc.Docker.Volumes...)
				}
			}
			for _, entry := range deps.Infra {
				if entry.Inline != nil {
					infraVolumes = append(infraVolumes, entry.Inline.Volumes...)
				}
			}
		}
	}

	if len(serviceVolumes) == 0 && len(infraVolumes) == 0 {
		output.PrintInfo(i18n.T("output.no_volumes_found"))
		return nil
	}

	// Normalize the same way as compose: service volumes with project name, infra volumes with workspace name
	seenNormalized := make(map[string]bool)
	normalizedVolumes := make([]string, 0)

	serviceNamed, err := uc.deps.DockerRunner.ExtractNamedVolumes(serviceVolumes)
	if err != nil {
		return errors.New(
			errors.ErrCodeDockerNotRunning,
			i18n.T("error.volumes_extract_services"),
		).WithError(err)
	}
	for _, volName := range serviceNamed {
		normalizedName, err := uc.deps.DockerRunner.NormalizeVolumeName(projectName, volName)
		if err != nil {
			logging.WarnWithContext(ctx, "Failed to normalize service volume name", "volume", volName, "error", err.Error())
			continue
		}
		if !seenNormalized[normalizedName] {
			seenNormalized[normalizedName] = true
			normalizedVolumes = append(normalizedVolumes, normalizedName)
		}
	}

	infraNamed, err := uc.deps.DockerRunner.ExtractNamedVolumes(infraVolumes)
	if err != nil {
		return errors.New(
			errors.ErrCodeDockerNotRunning,
			i18n.T("error.volumes_extract_infra"),
		).WithError(err)
	}
	for _, volName := range infraNamed {
		normalizedName, err := uc.deps.DockerRunner.NormalizeVolumeName(workspaceName, volName)
		if err != nil {
			logging.WarnWithContext(ctx, "Failed to normalize infra volume name", "volume", volName, "error", err.Error())
			continue
		}
		if !seenNormalized[normalizedName] {
			seenNormalized[normalizedName] = true
			normalizedVolumes = append(normalizedVolumes, normalizedName)
		}
	}

	if len(normalizedVolumes) == 0 {
		output.PrintInfo(i18n.T("output.no_volumes_to_remove"))
		return nil
	}

	// Get base directory for checking volume usage
	baseDir := uc.deps.Workspace.GetBaseDirFromWorkspace(ws)

	// Check which volumes are in use by other projects
	volumesToRemove := make([]string, 0)
	volumesInUse := make(map[string][]string) // volume -> list of projects using it

	for _, volName := range normalizedVolumes {
		volProjects, err := uc.deps.DockerRunner.GetVolumeProjects(volName, baseDir)
		if err != nil {
			logging.WarnWithContext(ctx, "Failed to check volume usage", "volume", volName, "error", err.Error())
			// Assume it's safe to remove if we can't check
			volumesToRemove = append(volumesToRemove, volName)
			continue
		}

		// Filter out current project
		otherProjects := make([]string, 0)
		for _, p := range volProjects {
			if p != projectName {
				otherProjects = append(otherProjects, p)
			}
		}

		if len(otherProjects) > 0 {
			volumesInUse[volName] = otherProjects
		} else {
			volumesToRemove = append(volumesToRemove, volName)
		}
	}

	// Show volumes that are in use
	if len(volumesInUse) > 0 {
		output.PrintWarning(i18n.T("output.volumes_in_use_warning", len(volumesInUse)))
		for volName, projects := range volumesInUse {
			output.PrintInfo(i18n.T("output.volume_detail", volName, strings.Join(projects, ", ")))
		}
		fmt.Println()
	}

	// Show volumes that can be removed
	if len(volumesToRemove) == 0 {
		output.PrintInfo(i18n.T("output.all_volumes_in_use"))
		return nil
	}

	output.PrintInfo(i18n.T("output.found_volumes_removable", len(volumesToRemove)))
	for _, volName := range volumesToRemove {
		output.PrintInfo(i18n.T("output.volume_item", volName))
	}
	fmt.Println()

	// Ask for confirmation unless --force is used
	if !opts.Force {
		fmt.Print(i18n.T("output.confirm_remove_volumes_prompt"))
		reader := bufio.NewReader(os.Stdin)
		response, err := reader.ReadString('\n')
		if err != nil {
			return errors.New(
				errors.ErrCodeWorkspaceError,
				i18n.T("error.read_input"),
			).WithError(err)
		}

		response = strings.TrimSpace(strings.ToLower(response))
		if response != "yes" && response != "y" {
			output.PrintInfo(i18n.T("output.volumes_cancelled"))
			return nil
		}
	}

	// Remove volumes
	output.PrintProgress(i18n.T("output.removing_volumes", len(volumesToRemove)))
	removedCount := 0
	failedCount := 0

	for _, volName := range volumesToRemove {
		logging.InfoWithContext(ctx, "Removing volume", "volume", volName)
		if err := uc.deps.DockerRunner.RemoveVolumeWithContext(ctx, volName); err != nil {
			logging.WarnWithContext(ctx, "Failed to remove volume", "volume", volName, "error", err.Error())
			output.PrintWarning(i18n.T("output.failed_remove_volume", volName, err))
			failedCount++
		} else {
			output.PrintSuccess(i18n.T("output.removed_volume", volName))
			removedCount++
		}
	}

	fmt.Println()
	if removedCount > 0 {
		output.PrintSuccess(i18n.T("output.volumes_removed_success", removedCount))
	}
	if failedCount > 0 {
		output.PrintWarning(i18n.T("output.volumes_removed_failed", failedCount))
	}

	logging.InfoWithContext(ctx, "Volumes removal completed",
		"removed", removedCount,
		"failed", failedCount,
	)

	return nil
}
