package upcase

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"raioz/internal/i18n"
	"raioz/internal/logging"
	"raioz/internal/output"
)

// checkAndHandleDuplicateProject checks if a project with the same name is running from workspace
// and handles it by doing down of the workspace project before starting the local one
func (uc *UseCase) checkAndHandleDuplicateProject(ctx context.Context, projectName string, configPath string) error {
	// Check if this is a local project
	isLocal, _, err := isLocalProject(configPath)
	if err != nil {
		return fmt.Errorf("failed to check if project is local: %w", err)
	}

	if !isLocal {
		// Not a local project, nothing to do
		return nil
	}

	// Load config to get workspace name
	deps, _, err := uc.deps.ConfigLoader.LoadDeps(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	workspaceName := deps.GetWorkspaceName()

	// Check if project is running from workspace
	ws, err := uc.deps.Workspace.Resolve(workspaceName)
	if err != nil {
		// Workspace doesn't exist or can't be resolved, no duplicate
		return nil
	}

	// ADR-011 Phase 2: liveness comes from Docker labels rather than
	// the legacy snapshot. IsProjectActive filters by both workspace and
	// project, so a true result implies the SAME project is running —
	// the old "load state, compare project name" check collapses into a
	// single label probe.
	active, err := uc.deps.DockerRunner.IsProjectActive(ctx, workspaceName, projectName)
	if err != nil {
		return fmt.Errorf("probe workspace project: %w", err)
	}
	if !active {
		return nil
	}

	// Project is running from workspace, need to ask for confirmation
	logging.InfoWithContext(ctx, "Project is running from workspace, asking for confirmation",
		"project", projectName,
		"workspace", uc.deps.Workspace.GetRoot(ws),
	)

	output.PrintWarning(i18n.T("up.duplicate.already_running", projectName))
	output.PrintInfo(i18n.T("up.duplicate.workspace_location", uc.deps.Workspace.GetRoot(ws)))
	output.PrintInfo(i18n.T("up.duplicate.will_stop_workspace"))
	output.PrintPrompt(i18n.T("up.duplicate.continue_prompt"))

	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read user response: %w", err)
	}

	response = strings.TrimSpace(strings.ToLower(response))
	if response != "yes" && response != "y" {
		output.PrintInfo(i18n.T("up.duplicate.cancelled"))
		output.PrintInfo(i18n.T("up.duplicate.stop_hint"))
		return fmt.Errorf("user cancelled: workspace project remains running")
	}

	output.PrintInfo(i18n.T("up.duplicate.stopping_workspace"))

	// Get compose path
	composePath := uc.deps.Workspace.GetComposePath(ws)

	// Stop Docker services
	if composePath != "" {
		output.PrintInfo(i18n.T("up.duplicate.stopping_docker"))
		if err := uc.deps.DockerRunner.DownWithContext(ctx, composePath); err != nil {
			logging.WarnWithContext(ctx, "Failed to stop Docker services from workspace", "error", err.Error())
			// Continue anyway - might already be stopped
		} else {
			output.PrintSuccess(i18n.T("up.duplicate.docker_stopped"))
		}
	}

	// Stop host processes if any
	hostProcesses, err := uc.deps.HostRunner.LoadProcessesState(ws)
	if err == nil && len(hostProcesses) > 0 {
		output.PrintInfo(i18n.T("up.duplicate.stopping_host", len(hostProcesses)))
		for name, processInfo := range hostProcesses {
			logging.InfoWithContext(ctx, "Stopping host service from workspace", "service", name, "pid", processInfo.PID)
			if err := uc.deps.HostRunner.StopServiceWithCommand(ctx, processInfo.PID, processInfo.StopCommand); err != nil {
				logging.WarnWithContext(ctx, "Failed to stop host service from workspace", "service", name, "error", err.Error())
			}
		}
		// Remove host processes state file
		if err := uc.deps.HostRunner.RemoveProcessesState(ws); err != nil {
			logging.WarnWithContext(ctx, "Failed to remove host processes state", "error", err.Error())
		}
	}

	// Remove state file
	statePath := uc.deps.Workspace.GetStatePath(ws)
	if err := os.Remove(statePath); err != nil && !os.IsNotExist(err) {
		logging.WarnWithContext(ctx, "Failed to remove workspace state file", "error", err.Error())
	}

	// Remove from global state
	if err := uc.deps.StateManager.RemoveProject(projectName); err != nil {
		logging.WarnWithContext(ctx, "Failed to remove project from global state", "error", err.Error())
	}

	output.PrintSuccess(i18n.T("up.duplicate.project_stopped", projectName))
	return nil
}
