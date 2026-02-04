package app

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"raioz/internal/config"
	"raioz/internal/docker"
	"raioz/internal/domain/interfaces"
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
	logging.LogOperationStart(ctx, "raioz down",
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
			"Check that the project name is correct. "+
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
			"Wait for the other process to finish, "+
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

			// If no stopCommand, try to detect docker-compose.yml
			if stopCommand == "" {
				var composePathToUse string

				// First, try to use ComposePath from ProcessInfo (saved when service was started)
				if processInfo.ComposePath != "" {
					// Verify the compose file exists
					if _, err := os.Stat(processInfo.ComposePath); err == nil {
						composePathToUse = processInfo.ComposePath
						logging.InfoWithContext(ctx, "Using ComposePath from ProcessInfo", "service", name, "composePath", composePathToUse)
					} else {
						logging.WarnWithContext(ctx, "ComposePath from ProcessInfo does not exist, trying to detect", "service", name, "composePath", processInfo.ComposePath, "error", err.Error())
					}
				}

				// If ComposePath from ProcessInfo doesn't exist or wasn't set, try to detect it
				if composePathToUse == "" && currentDeps != nil {
					// If not in ProcessInfo, try to detect it from current config
					if svc, exists := currentDeps.Services[name]; exists {
						// Get service path if not already set
						if servicePath == "" && svc.Source.Kind == "git" {
							servicePath = workspacepkg.GetServicePath(wsConcrete, name, svc)
						}

						// Try to detect compose path
						if servicePath != "" {
							var explicitComposePath string
							if svc.Commands != nil {
								explicitComposePath = svc.Commands.ComposePath
							}
							// Get command for detection (try source.command first, then commands.up)
							command := ""
							if svc.Source.Command != "" {
								command = svc.Source.Command
							} else if svc.Commands != nil && svc.Commands.Up != "" {
								command = svc.Commands.Up
							}
							composePathToUse = host.DetectComposePath(servicePath, command, explicitComposePath)
							if composePathToUse != "" {
								logging.InfoWithContext(ctx, "Detected docker-compose.yml for host service", "service", name, "composePath", composePathToUse, "servicePath", servicePath)
							}
						}
					}
				}

				// If composePath is found, use docker-compose down
				if composePathToUse != "" {
					composeDir := filepath.Dir(composePathToUse)
					// Use absolute path for docker compose -f flag
					stopCommand = fmt.Sprintf("docker compose -f %s down", composePathToUse)
					if servicePath == "" {
						servicePath = composeDir
					}
					logging.InfoWithContext(ctx, "Using docker-compose down for host service", "service", name, "composePath", composePathToUse, "composeDir", composeDir)
				}
			}

			// Always try to stop the service, even if no stopCommand is provided
			// StopServiceWithCommandAndPath will kill the process by PID if stopCommand is empty
			logging.InfoWithContext(ctx, "Stopping host service", "service", name, "pid", processInfo.PID, "stopCommand", stopCommand, "servicePath", servicePath)
			if err := host.StopServiceWithCommandAndPath(ctx, processInfo.PID, stopCommand, servicePath); err != nil {
				logging.WarnWithContext(ctx, "Failed to stop host service", "service", name, "pid", processInfo.PID, "error", err.Error())
				output.PrintWarning(fmt.Sprintf("Failed to stop host service %s (PID: %d): %v", name, processInfo.PID, err))
			} else {
				if stopCommand != "" {
					output.PrintSuccess(fmt.Sprintf("Stopped host service %s (PID: %d) using stop command", name, processInfo.PID))
				} else {
					output.PrintSuccess(fmt.Sprintf("Stopped host service %s (PID: %d)", name, processInfo.PID))
				}
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
			"The state file may be corrupted. "+
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
			"Check Docker daemon status with 'docker ps'. "+
				"Verify that Docker Compose is installed and working. "+
				"Services may already be stopped.",
		).WithContext("compose_file", composePath).WithError(err)
	}

	// Check if network is still in use by other projects
	// We leave the network for reusability (idempotence), but check usage
	networkName := stateDeps.Network.GetName()
	baseDir := uc.deps.Workspace.GetBaseDirFromWorkspace(ws)

	// First, check if network is actually in use by containers (most reliable check)
	isInUse, err := docker.IsNetworkInUseWithContext(ctx, networkName)
	if err != nil {
		// Log but don't fail - network cleanup is optional
		logging.WarnWithContext(ctx, "could not check if network is in use by containers", "error", err)
		isInUse = false // Assume not in use if we can't check
	}

	// Get workspace name for comparison (GetNetworkProjects returns workspace directory names)
	// The workspace name is what's used as the directory name, not the project name
	currentWorkspaceName := workspaceName
	if currentWorkspaceName == "" {
		// Fallback: use project name if workspace name wasn't determined
		currentWorkspaceName = projectName
	}

	// Also check state files to see if other projects are configured to use this network
	networkProjects, err := uc.deps.DockerRunner.GetNetworkProjects(networkName, baseDir)
	if err != nil {
		// Log but don't fail - network cleanup is optional
		logging.WarnWithContext(ctx, "could not check network usage from state files", "error", err)
	}

	// Count remaining projects using network (excluding current workspace)
	// Note: GetNetworkProjects returns workspace directory names, not project names
	// Also exclude the current workspace since we're about to remove its state file
	remainingNetworkProjects := 0
	for _, workspaceDirName := range networkProjects {
		if workspaceDirName != currentWorkspaceName {
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
			"Check file permissions. "+
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
	// Check if project has docker-compose.yml in project directory
	// First, try to use ProjectComposePath from state (if saved during up)
	var projectComposePath string
	if stateDeps.ProjectComposePath != "" {
		projectComposePath = stateDeps.ProjectComposePath
	} else if opts.ConfigPath != "" {
		// Fallback: detect docker-compose.yml in project directory
		absConfigPath, err := filepath.Abs(opts.ConfigPath)
		if err == nil {
			projectDir := filepath.Dir(absConfigPath)

			// Check for docker-compose.yml in project directory
			composeFiles := []string{
				filepath.Join(projectDir, "docker-compose.yml"),
				filepath.Join(projectDir, "docker-compose.yaml"),
				filepath.Join(projectDir, "compose.yml"),
				filepath.Join(projectDir, "compose.yaml"),
			}

			for _, composeFile := range composeFiles {
				if _, err := os.Stat(composeFile); err == nil {
					projectComposePath = composeFile
					break
				}
			}
		}
	}

	// If docker-compose.yml found, stop it
	if projectComposePath != "" {
		output.PrintInfo(fmt.Sprintf("Stopping Docker Compose services from project directory..."))
		logging.InfoWithContext(ctx, "Found docker-compose.yml in project directory, stopping it", "composePath", projectComposePath)
		if err := uc.deps.DockerRunner.DownWithContext(ctx, projectComposePath); err != nil {
			logging.WarnWithContext(ctx, "Failed to stop Docker Compose services from project directory", "error", err.Error())
			output.PrintWarning("Failed to stop Docker Compose services from project directory (may already be stopped)")
		} else {
			output.PrintSuccess("Docker Compose services stopped from project directory")
		}
	}

	// Execute project.commands.down if defined
	if stateDeps.Project.Commands != nil {
		var downCommand string
		mode := "dev"
		// Try to get mode from first service with docker config
		for _, svc := range stateDeps.Services {
			if svc.Docker != nil && svc.Docker.Mode != "" {
				mode = svc.Docker.Mode
				break
			}
		}

		if mode == "prod" && stateDeps.Project.Commands.Prod != nil && stateDeps.Project.Commands.Prod.Down != "" {
			downCommand = stateDeps.Project.Commands.Prod.Down
		} else if mode == "dev" && stateDeps.Project.Commands.Dev != nil && stateDeps.Project.Commands.Dev.Down != "" {
			downCommand = stateDeps.Project.Commands.Dev.Down
		} else if stateDeps.Project.Commands.Down != "" {
			downCommand = stateDeps.Project.Commands.Down
		}

		if downCommand != "" {
			// Get project directory from config path or state
			var projectDir string
			if opts.ConfigPath != "" {
				absConfigPath, err := filepath.Abs(opts.ConfigPath)
				if err == nil {
					projectDir = filepath.Dir(absConfigPath)
				}
			}
			if projectDir == "" {
				// Fallback: try to get from workspace
				projectDir = uc.deps.Workspace.GetRoot(ws)
			}

			output.PrintInfo(fmt.Sprintf("Executing project down command: %s", downCommand))
			logging.InfoWithContext(ctx, "Executing project down command", "command", downCommand, "projectDir", projectDir)

			cmdParts := strings.Fields(downCommand)
			if len(cmdParts) > 0 {
				var cmd *exec.Cmd
				if len(cmdParts) == 1 {
					cmd = exec.CommandContext(ctx, cmdParts[0])
				} else {
					cmd = exec.CommandContext(ctx, cmdParts[0], cmdParts[1:]...)
				}
				cmd.Dir = projectDir
				cmd.Env = os.Environ()
				cmd.Env = append(cmd.Env, fmt.Sprintf("RAIOZ_MODE=%s", mode))
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr

				if err := cmd.Run(); err != nil {
					logging.WarnWithContext(ctx, "Failed to execute project down command", "error", err.Error())
					output.PrintWarning(fmt.Sprintf("Failed to execute project down command (may already be stopped): %v", err))
				} else {
					output.PrintSuccess("Project down command executed successfully")
				}
			}
		} else if len(stateDeps.Services) == 0 && stateDeps.Project.Commands.Up != "" {
			// Command-only project with no project.commands.down: try to stop containers that may belong to this project
			// (e.g. started by project.commands.up like "make deploy-nginx" which may create a container named "nginx")
			projName := stateDeps.Project.Name
			hasExplicit := stateDeps.HasExplicitWorkspace()
			wsName := currentWorkspaceName
			if wsName == "" {
				wsName = projName
			}
			containerNames := []string{projName}
			if normalized, err := docker.NormalizeContainerName(wsName, projName, projName, hasExplicit); err == nil && normalized != projName {
				containerNames = append([]string{normalized}, containerNames...)
			}
			for _, name := range containerNames {
				if err := docker.StopContainerWithContext(ctx, name); err != nil {
					logging.WarnWithContext(ctx, "Failed to stop container by name", "container", name, "error", err.Error())
				} else {
					output.PrintSuccess(fmt.Sprintf("Stopped container %s", name))
					break
				}
			}
		}
	}

	output.PrintSuccess(fmt.Sprintf("Project '%s' stopped successfully", stateDeps.Project.Name))

	// Note about network (we leave it for reuse, better for idempotence)
	// Only show message if network is actually in use by containers OR other projects are configured to use it
	// If network is not in use and no other projects use it, don't show any message
	if remainingNetworkProjects > 0 {
		output.PrintInfo(fmt.Sprintf("Network '%s' is still in use by %d other project(s), leaving it",
			networkName, remainingNetworkProjects))
	} else if isInUse {
		// Network is in use by containers (but not by other raioz projects)
		output.PrintInfo(fmt.Sprintf("Network '%s' is still in use by containers, leaving it",
			networkName))
	}
	// If not in use and no other projects, don't show any message (network is truly unused)

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
