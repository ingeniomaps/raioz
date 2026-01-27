package app

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"raioz/internal/config"
	"raioz/internal/domain/interfaces"
	"raioz/internal/docker"
	"raioz/internal/errors"
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
				"Could not determine project name",
			).WithSuggestion(
				"Please provide --config or --project flag to specify the project.",
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
			"Failed to resolve workspace",
		).WithSuggestion(
			"Check that the project name is correct. " +
				"Verify workspace directories exist and are accessible.",
		).WithContext("project", projectName).WithError(err)
	}

	// Get workspace root using interface method
	wsRoot := uc.deps.Workspace.GetRoot(ws)
	logging.InfoWithContext(ctx, "Workspace resolved",
		"workspace", wsRoot,
	)

	// Try to load state to get volumes from state
	var allVolumes []string
	var stateDeps *config.Deps

	if uc.deps.StateManager.Exists(ws) {
		stateDeps, err = uc.deps.StateManager.Load(ws)
		if err == nil {
			// Collect volumes from state
			for _, svc := range stateDeps.Services {
				if svc.Docker != nil {
					allVolumes = append(allVolumes, svc.Docker.Volumes...)
				}
			}
			for _, infra := range stateDeps.Infra {
				allVolumes = append(allVolumes, infra.Volumes...)
			}
		}
	}

	// If no state, try to load from config
	if len(allVolumes) == 0 {
		deps, _, _ := uc.deps.ConfigLoader.LoadDeps(opts.ConfigPath)
		if deps != nil {
			stateDeps = deps
			for _, svc := range deps.Services {
				if svc.Docker != nil {
					allVolumes = append(allVolumes, svc.Docker.Volumes...)
				}
			}
			for _, infra := range deps.Infra {
				allVolumes = append(allVolumes, infra.Volumes...)
			}
		}
	}

	if len(allVolumes) == 0 {
		output.PrintInfo("No volumes found for this project")
		return nil
	}

	// Extract named volumes
	originalNamedVolumes, err := docker.ExtractNamedVolumes(allVolumes)
	if err != nil {
		return errors.New(
			errors.ErrCodeDockerNotRunning,
			"Failed to extract named volumes",
		).WithError(err)
	}

	if len(originalNamedVolumes) == 0 {
		output.PrintInfo("No named volumes found for this project")
		return nil
	}

	// Normalize volume names with project prefix
	normalizedVolumes := make([]string, 0, len(originalNamedVolumes))
	for _, volName := range originalNamedVolumes {
		normalizedName, err := docker.NormalizeVolumeName(projectName, volName)
		if err != nil {
			logging.WarnWithContext(ctx, "Failed to normalize volume name", "volume", volName, "error", err.Error())
			continue
		}
		normalizedVolumes = append(normalizedVolumes, normalizedName)
	}

	if len(normalizedVolumes) == 0 {
		output.PrintInfo("No volumes to remove")
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
		output.PrintWarning(fmt.Sprintf("⚠️  %d volume(s) are in use by other projects and will not be removed:", len(volumesInUse)))
		for volName, projects := range volumesInUse {
			output.PrintInfo(fmt.Sprintf("  - %s (used by: %s)", volName, strings.Join(projects, ", ")))
		}
		fmt.Println()
	}

	// Show volumes that can be removed
	if len(volumesToRemove) == 0 {
		output.PrintInfo("All volumes are in use by other projects. Nothing to remove.")
		return nil
	}

	output.PrintInfo(fmt.Sprintf("Found %d volume(s) that can be removed:", len(volumesToRemove)))
	for _, volName := range volumesToRemove {
		output.PrintInfo(fmt.Sprintf("  - %s", volName))
	}
	fmt.Println()

	// Ask for confirmation unless --force is used
	if !opts.Force {
		fmt.Print("Do you want to remove these volumes? (yes/no): ")
		reader := bufio.NewReader(os.Stdin)
		response, err := reader.ReadString('\n')
		if err != nil {
			return errors.New(
				errors.ErrCodeWorkspaceError,
				"Failed to read user response",
			).WithError(err)
		}

		response = strings.TrimSpace(strings.ToLower(response))
		if response != "yes" && response != "y" {
			output.PrintInfo("Operation cancelled. Volumes were not removed.")
			return nil
		}
	}

	// Remove volumes
	output.PrintProgress(fmt.Sprintf("Removing %d volume(s)...", len(volumesToRemove)))
	removedCount := 0
	failedCount := 0

	for _, volName := range volumesToRemove {
		logging.InfoWithContext(ctx, "Removing volume", "volume", volName)
		if err := docker.RemoveVolumeWithContext(ctx, volName); err != nil {
			logging.WarnWithContext(ctx, "Failed to remove volume", "volume", volName, "error", err.Error())
			output.PrintWarning(fmt.Sprintf("Failed to remove volume '%s': %v", volName, err))
			failedCount++
		} else {
			output.PrintSuccess(fmt.Sprintf("Removed volume '%s'", volName))
			removedCount++
		}
	}

	fmt.Println()
	if removedCount > 0 {
		output.PrintSuccess(fmt.Sprintf("Successfully removed %d volume(s)", removedCount))
	}
	if failedCount > 0 {
		output.PrintWarning(fmt.Sprintf("Failed to remove %d volume(s)", failedCount))
	}

	logging.InfoWithContext(ctx, "Volumes removal completed",
		"removed", removedCount,
		"failed", failedCount,
	)

	return nil
}
