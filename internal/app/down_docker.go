package app

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"raioz/internal/config"
	"raioz/internal/domain/interfaces"
	"raioz/internal/logging"
	"raioz/internal/output"
)

// handleNetworkAndVolumes checks network and volume usage after docker compose down.
// Returns remainingNetworkProjects count and whether the network is in use by containers.
func (uc *DownUseCase) handleNetworkAndVolumes(
	ctx context.Context,
	stateDeps *config.Deps,
	ws *interfaces.Workspace,
	projectName, workspaceName string,
) (int, bool) {
	networkName := stateDeps.Network.GetName()
	baseDir := uc.deps.Workspace.GetBaseDirFromWorkspace(ws)

	isInUse, err := uc.deps.DockerRunner.IsNetworkInUseWithContext(ctx, networkName)
	if err != nil {
		logging.WarnWithContext(ctx, "could not check if network is in use by containers", "error", err)
		isInUse = false
	}

	currentWorkspaceName := workspaceName
	if currentWorkspaceName == "" {
		currentWorkspaceName = projectName
	}

	networkProjects, err := uc.deps.DockerRunner.GetNetworkProjects(networkName, baseDir)
	if err != nil {
		logging.WarnWithContext(ctx, "could not check network usage from state files", "error", err)
	}

	remainingNetworkProjects := 0
	for _, workspaceDirName := range networkProjects {
		if workspaceDirName != currentWorkspaceName {
			remainingNetworkProjects++
		}
	}

	// Check named volumes usage
	var allVolumes []string
	for _, svc := range stateDeps.Services {
		if svc.Docker != nil {
			allVolumes = append(allVolumes, svc.Docker.Volumes...)
		}
	}
	for _, entry := range stateDeps.Infra {
		if entry.Inline != nil {
			allVolumes = append(allVolumes, entry.Inline.Volumes...)
		}
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

	return remainingNetworkProjects, isInUse
}

// handleProjectComposeDown stops docker-compose services from the project directory.
func (uc *DownUseCase) handleProjectComposeDown(ctx context.Context, stateDeps *config.Deps, opts DownOptions) {
	var projectComposePath string
	if stateDeps.ProjectComposePath != "" {
		projectComposePath = stateDeps.ProjectComposePath
	} else if opts.ConfigPath != "" {
		absConfigPath, err := filepath.Abs(opts.ConfigPath)
		if err == nil {
			projectDir := filepath.Dir(absConfigPath)
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

	if projectComposePath == "" {
		return
	}

	output.PrintInfo("Stopping Docker Compose services from project directory...")
	logging.InfoWithContext(ctx, "Found docker-compose.yml in project directory, stopping it", "composePath", projectComposePath)
	if err := uc.deps.DockerRunner.DownWithContext(ctx, projectComposePath); err != nil {
		logging.WarnWithContext(ctx, "Failed to stop Docker Compose services from project directory", "error", err.Error())
		output.PrintWarning("Failed to stop Docker Compose services from project directory (may already be stopped)")
	} else {
		output.PrintSuccess("Docker Compose services stopped from project directory")
	}
}

// executeProjectDownCommand executes the project.commands.down if defined.
func (uc *DownUseCase) executeProjectDownCommand(
	ctx context.Context,
	stateDeps *config.Deps,
	ws *interfaces.Workspace,
	opts DownOptions,
	workspaceName string,
) {
	if stateDeps.Project.Commands == nil {
		return
	}

	var downCommand string
	mode := "dev"
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
		uc.runDownCommand(ctx, downCommand, mode, ws, opts)
	} else if len(stateDeps.Services) == 0 && stateDeps.Project.Commands.Up != "" {
		uc.stopCommandOnlyProjectContainers(ctx, stateDeps, workspaceName)
	}
}

// runDownCommand executes a project down command string.
func (uc *DownUseCase) runDownCommand(ctx context.Context, downCommand, mode string, ws *interfaces.Workspace, opts DownOptions) {
	var projectDir string
	if opts.ConfigPath != "" {
		absConfigPath, err := filepath.Abs(opts.ConfigPath)
		if err == nil {
			projectDir = filepath.Dir(absConfigPath)
		}
	}
	if projectDir == "" {
		projectDir = uc.deps.Workspace.GetRoot(ws)
	}

	output.PrintInfo(fmt.Sprintf("Executing project down command: %s", downCommand))
	logging.InfoWithContext(ctx, "Executing project down command", "command", downCommand, "projectDir", projectDir)

	cmdParts := strings.Fields(downCommand)
	if len(cmdParts) == 0 {
		return
	}

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

// stopCommandOnlyProjectContainers attempts to stop containers for command-only projects.
func (uc *DownUseCase) stopCommandOnlyProjectContainers(ctx context.Context, stateDeps *config.Deps, workspaceName string) {
	projName := stateDeps.Project.Name
	hasExplicit := stateDeps.HasExplicitWorkspace()
	wsName := workspaceName
	if wsName == "" {
		wsName = projName
	}
	containerNames := []string{projName}
	if normalized, err := uc.deps.DockerRunner.NormalizeContainerName(wsName, projName, projName, hasExplicit); err == nil && normalized != projName {
		containerNames = append([]string{normalized}, containerNames...)
	}
	for _, name := range containerNames {
		if err := uc.deps.DockerRunner.StopContainerWithContext(ctx, name); err != nil {
			logging.WarnWithContext(ctx, "Failed to stop container by name", "container", name, "error", err.Error())
		} else {
			output.PrintSuccess(fmt.Sprintf("Stopped container %s", name))
			break
		}
	}
}

// cleanupDockerResources cleans unused Docker images and volumes after down.
func (uc *DownUseCase) cleanupDockerResources(ctx context.Context) {
	output.PrintProgress("Cleaning up unused Docker images and volumes...")

	imageActions, err := uc.deps.DockerRunner.CleanUnusedImagesWithContext(ctx, false)
	if err != nil {
		logging.WarnWithContext(ctx, "Failed to clean unused images", "error", err.Error())
		output.PrintProgressError("Failed to clean unused images")
	} else {
		for _, action := range imageActions {
			logging.DebugWithContext(ctx, "Image cleanup action", "action", action)
		}
		output.PrintProgressDone("Unused images cleaned")
	}

	volumeActions, err := uc.deps.DockerRunner.CleanUnusedVolumesWithContext(ctx, false, true)
	if err != nil {
		logging.WarnWithContext(ctx, "Failed to clean unused volumes", "error", err.Error())
		output.PrintProgressError("Failed to clean unused volumes")
	} else {
		for _, action := range volumeActions {
			logging.DebugWithContext(ctx, "Volume cleanup action", "action", action)
		}
		output.PrintProgressDone("Unused volumes cleaned")
	}
}
