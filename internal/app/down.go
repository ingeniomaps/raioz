package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"raioz/internal/config"
	"raioz/internal/domain/interfaces"
	"raioz/internal/docker"
	"raioz/internal/errors"
	"raioz/internal/host"
	"raioz/internal/logging"
	"raioz/internal/output"
	workspacepkg "raioz/internal/workspace"
)

// DownOptions contains options for the Down use case
type DownOptions struct {
	ProjectName string
	ConfigPath  string
}

// DownUseCase handles the "down" use case - stopping a project
type DownUseCase struct {
	deps *Dependencies
}

// NewDownUseCase creates a new DownUseCase with injected dependencies
func NewDownUseCase(deps *Dependencies) *DownUseCase {
	return &DownUseCase{
		deps: deps,
	}
}

// Execute executes the down use case
func (uc *DownUseCase) Execute(ctx context.Context, opts DownOptions) error {
	// Add request ID and operation context for logging correlation
	ctx = logging.WithRequestID(ctx)
	ctx = logging.WithOperation(ctx, "raioz down")

	var ws *interfaces.Workspace
	var err error

	// Determine project name
	projectName := opts.ProjectName
	if projectName == "" {
		logging.DebugWithContext(ctx, "Project name not provided, loading from config",
			"config_path", opts.ConfigPath,
		)
		deps, _, _ := uc.deps.ConfigLoader.LoadDeps(opts.ConfigPath)
		if deps != nil {
			projectName = deps.Project.Name
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
	}

	// Log operation start
	logging.LogOperationStart(ctx, "raioz down",
		"project", projectName,
	)

	// Resolve workspace
	ws, err = uc.deps.Workspace.Resolve(projectName)
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

	// Perform validation before down
	if err := uc.deps.Validator.ValidateBeforeDown(ctx, ws); err != nil {
		logging.ErrorWithContext(ctx, "Validation failed",
			"error", err.Error(),
		)
		return err
	}

	// Acquire lock
		// Only log at debug level - not useful for end users
		logging.DebugWithContext(ctx, "Acquiring lock",
		"workspace", wsRoot,
	)
	lockInstance, err := uc.deps.LockManager.Acquire(ws)
	if err != nil {
		logging.ErrorWithContext(ctx, "Failed to acquire lock",
			"workspace", wsRoot,
			"error", err.Error(),
		)
			return errors.New(
				errors.ErrCodeLockError,
				"Failed to acquire lock (another raioz process may be running)",
			).WithSuggestion(
				"Wait for the other process to finish, " +
					"or remove the lock file manually if the process crashed.",
			).WithContext("workspace", wsRoot).WithError(err)
		}
		// Only log at debug level - not useful for end users
		logging.DebugWithContext(ctx, "Lock acquired successfully")
	defer func() {
		if err := lockInstance.Release(); err != nil {
			logging.ErrorWithContext(ctx, "Failed to release lock",
				"error", err.Error(),
			)
		} else {
			// Only log at debug level - not useful for end users
			logging.DebugWithContext(ctx, "Lock released")
		}
	}()

	// Stop host processes first (before checking state)
	// This allows stopping host services even if state file is missing
	// Convert interfaces.Workspace to concrete workspace.Workspace
	wsConcrete := (*workspacepkg.Workspace)(ws)
	hostProcesses, err := host.LoadProcessesState(wsConcrete)
	if err != nil {
		logging.WarnWithContext(ctx, "Failed to load host processes state", "error", err.Error())
	} else if len(hostProcesses) > 0 {
		// Load current config to get stopCommand if not in state
		var currentDeps *config.Deps
		if opts.ConfigPath != "" {
			currentDeps, _, _ = uc.deps.ConfigLoader.LoadDeps(opts.ConfigPath)
		}

		output.PrintInfo(fmt.Sprintf("Stopping %d host service(s)...", len(hostProcesses)))
		for name, processInfo := range hostProcesses {
			stopCommand := processInfo.StopCommand
			var servicePath string

			// If stopCommand is not in state, try to get it from current config
			if stopCommand == "" && currentDeps != nil {
				if svc, exists := currentDeps.Services[name]; exists && svc.Commands != nil {
					mode := "dev"
					if svc.Docker != nil && svc.Docker.Mode != "" {
						mode = svc.Docker.Mode
					}

					// Get stop command based on mode
					if mode == "prod" && svc.Commands.Prod != nil && svc.Commands.Prod.Down != "" {
						stopCommand = svc.Commands.Prod.Down
					} else if mode == "dev" && svc.Commands.Dev != nil && svc.Commands.Dev.Down != "" {
						stopCommand = svc.Commands.Dev.Down
					} else if svc.Commands.Down != "" {
						stopCommand = svc.Commands.Down
					}

					// Get service path for stop command execution
					if svc.Source.Kind == "git" {
						servicePath = workspacepkg.GetServicePath(wsConcrete, name, svc)
					}

					if stopCommand != "" {
						logging.InfoWithContext(ctx, "Using stopCommand from current config", "service", name, "stopCommand", stopCommand)
					}
				}
			} else if currentDeps != nil {
				// Get service path even if stopCommand is already in state
				if svc, exists := currentDeps.Services[name]; exists && svc.Source.Kind == "git" {
					servicePath = workspacepkg.GetServicePath(wsConcrete, name, svc)
				}
			}

			// If no stopCommand but composePath is available, use docker-compose down
			if stopCommand == "" && processInfo.ComposePath != "" {
				// Use docker-compose down for the detected compose file
				composeDir := filepath.Dir(processInfo.ComposePath)
				stopCommand = fmt.Sprintf("docker-compose -f %s down", filepath.Base(processInfo.ComposePath))
				if servicePath == "" {
					servicePath = composeDir
				}
				logging.InfoWithContext(ctx, "Using docker-compose down for host service", "service", name, "composePath", processInfo.ComposePath)
			}

			logging.InfoWithContext(ctx, "Stopping host service", "service", name, "pid", processInfo.PID, "stopCommand", stopCommand, "servicePath", servicePath)
			if err := host.StopServiceWithCommandAndPath(ctx, processInfo.PID, stopCommand, servicePath); err != nil {
				logging.WarnWithContext(ctx, "Failed to stop host service", "service", name, "pid", processInfo.PID, "error", err.Error())
			} else {
				output.PrintSuccess(fmt.Sprintf("Stopped host service %s (PID: %d)", name, processInfo.PID))
			}
		}
		// Remove host processes state file
		if err := host.RemoveProcessesState(wsConcrete); err != nil {
			logging.WarnWithContext(ctx, "Failed to remove host processes state", "error", err.Error())
		}
	}

	// Check if state exists
	if !uc.deps.StateManager.Exists(ws) {
		logging.WarnWithContext(ctx, "Project state not found",
			"workspace", wsRoot,
		)
		// If we stopped host services, that's enough - return success
		if len(hostProcesses) > 0 {
			output.PrintInfo("ℹ️  No state file found, but host services have been stopped")
			return nil
		}
		// No host services and no state - return error
		return errors.New(
			errors.ErrCodeStateLoadError,
			"Project is not running (no state file found)",
		).WithSuggestion(
			"The project may not have been started yet. " +
				"Run 'raioz up' to start the project.",
		)
	}

	// Load state to get compose path
	logging.DebugWithContext(ctx, "Loading project state",
		"workspace", wsRoot,
	)
	stateDeps, err := uc.deps.StateManager.Load(ws)
	if err != nil {
		logging.ErrorWithContext(ctx, "Failed to load project state",
			"workspace", wsRoot,
			"error", err.Error(),
		)
			return errors.New(
				errors.ErrCodeStateLoadError,
				"Failed to load project state",
			).WithSuggestion(
				"The state file may be corrupted. " +
					"You can try removing the state file manually.",
			).WithContext("workspace", wsRoot).WithError(err)
		}
		logging.InfoWithContext(ctx, "Project state loaded",
			"project", stateDeps.Project.Name,
			"services_count", len(stateDeps.Services),
		)

	composePath := uc.deps.Workspace.GetComposePath(ws)

	output.PrintInfo("Stopping Docker services...")
	if err := uc.deps.DockerRunner.DownWithContext(ctx, composePath); err != nil {
			return errors.New(
				errors.ErrCodeDockerNotRunning,
				"Failed to stop Docker Compose services",
			).WithSuggestion(
				"Check Docker daemon status with 'docker ps'. " +
					"Verify that Docker Compose is installed and working. " +
					"Services may already be stopped.",
			).WithContext("compose_file", composePath).WithError(err)
		}

	// Check if network is still in use by other projects
	// We leave the network for reusability (idempotence), but check usage
	networkName := stateDeps.Project.Network
	baseDir := uc.deps.Workspace.GetBaseDirFromWorkspace(ws)
	networkProjects, err := uc.deps.DockerRunner.GetNetworkProjects(networkName, baseDir)
	if err != nil {
		// Log but don't fail - network cleanup is optional
		logging.Warn("could not check network usage", "error", err)
	}

	// Count remaining projects using network (excluding current one)
	remainingNetworkProjects := 0
	for _, p := range networkProjects {
		if p != projectName {
			remainingNetworkProjects++
		}
	}

	// Check named volumes usage
	var allVolumes []string
	for _, svc := range stateDeps.Services {
		// Skip services without Docker configuration (host services)
		if svc.Docker != nil {
			allVolumes = append(allVolumes, svc.Docker.Volumes...)
		}
	}
	for _, infra := range stateDeps.Infra {
		allVolumes = append(allVolumes, infra.Volumes...)
	}

	namedVolumes, err := uc.deps.DockerRunner.ExtractNamedVolumes(allVolumes)
	if err == nil {
		for _, volName := range namedVolumes {
			volProjects, err := uc.deps.DockerRunner.GetVolumeProjects(volName, baseDir)
			if err == nil {
				remainingVolProjects := 0
				for _, p := range volProjects {
					if p != projectName {
						remainingVolProjects++
					}
				}
				if remainingVolProjects > 0 {
					output.PrintInfo(fmt.Sprintf("Volume '%s' is still in use by %d other project(s), leaving it",
						volName, remainingVolProjects))
				}
			}
		}
	}

	// Remove state file
	statePath := uc.deps.Workspace.GetStatePath(ws)
	if err := os.Remove(statePath); err != nil && !os.IsNotExist(err) {
		return errors.New(
			errors.ErrCodeStateSaveError,
			"Failed to remove state file",
		).WithSuggestion(
			"Check file permissions. " +
				"You can try removing the state file manually.",
		).WithContext("state_path", statePath).WithError(err)
	}

	// Update global state - remove project or mark services as stopped
	// We remove the project from global state when it's stopped
	if err := uc.deps.StateManager.RemoveProject(projectName); err != nil {
		// Log but don't fail - global state is optional
		logging.Warn("failed to update global state", "error", err)
	}

	// Execute local project down command if this is a local project
	// This is handled by a separate function to avoid import cycles
	// For now, we'll skip this in down.go and handle it in the command layer if needed

	output.PrintSuccess(fmt.Sprintf("Project '%s' stopped successfully", stateDeps.Project.Name))

	// Note about network (we leave it for reuse, better for idempotence)
	if remainingNetworkProjects > 0 {
		output.PrintInfo(fmt.Sprintf("Network '%s' is still in use by %d other project(s), leaving it",
			networkName, remainingNetworkProjects))
	} else {
		output.PrintInfo(fmt.Sprintf("Network '%s' is not in use by other projects, leaving it for reuse",
			networkName))
	}

	// Clean up unused Docker images and volumes
	output.PrintProgress("Cleaning up unused Docker images and volumes...")

	// Clean unused images
	imageActions, err := docker.CleanUnusedImagesWithContext(ctx, false)
	if err != nil {
		logging.WarnWithContext(ctx, "Failed to clean unused images", "error", err.Error())
		output.PrintProgressError("Failed to clean unused images")
	} else {
		if len(imageActions) > 0 {
			for _, action := range imageActions {
				logging.DebugWithContext(ctx, "Image cleanup action", "action", action)
			}
		}
		output.PrintProgressDone("Unused images cleaned")
	}

	// Clean unused volumes (with force=true to actually remove them)
	volumeActions, err := docker.CleanUnusedVolumesWithContext(ctx, false, true)
	if err != nil {
		logging.WarnWithContext(ctx, "Failed to clean unused volumes", "error", err.Error())
		output.PrintProgressError("Failed to clean unused volumes")
	} else {
		if len(volumeActions) > 0 {
			for _, action := range volumeActions {
				logging.DebugWithContext(ctx, "Volume cleanup action", "action", action)
			}
		}
		output.PrintProgressDone("Unused volumes cleaned")
	}

	logging.InfoWithContext(ctx, "Project stopped successfully",
		"project", projectName,
	)

	return nil
}
