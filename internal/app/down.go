package app

import (
	"context"
	"os"

	"raioz/internal/domain/interfaces"
	"raioz/internal/errors"
	"raioz/internal/i18n"
	"raioz/internal/logging"
	"raioz/internal/output"
)

// DownOptions contains options for the Down use case
type DownOptions struct {
	ProjectName string
	ConfigPath  string
	All         bool
	PruneShared bool
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
	ctx = logging.WithRequestID(ctx)
	ctx = logging.WithOperation(ctx, "raioz down")

	projectName, workspaceName, err := uc.resolveProject(ctx, opts)
	if err != nil {
		return err
	}
	ctx = logging.WithProject(ctx, projectName)

	logging.LogOperationStart(ctx, "raioz down", "project", projectName)

	ws, wsRoot, err := uc.resolveWorkspace(ctx, workspaceName, projectName)
	if err != nil {
		return err
	}

	if err := uc.deps.Validator.ValidateBeforeDown(ctx, ws); err != nil {
		logging.ErrorWithContext(ctx, "Validation failed", "error", err.Error())
		return err
	}

	lockInstance, err := uc.acquireLock(ctx, ws, wsRoot)
	if err != nil {
		return err
	}
	defer func() {
		if err := lockInstance.Release(); err != nil {
			logging.ErrorWithContext(ctx, "Failed to release lock", "error", err.Error())
		} else {
			logging.DebugWithContext(ctx, "Lock released")
		}
	}()

	// Stop host processes first (before checking state)
	hostProcesses := uc.stopHostProcesses(ctx, ws, opts)

	// Check if state exists
	if !uc.deps.StateManager.Exists(ws) {
		logging.WarnWithContext(ctx, "Project state not found", "workspace", wsRoot)
		if len(hostProcesses) > 0 {
			output.PrintInfo(i18n.T("output.no_state_host_stopped"))
			return nil
		}
		output.PrintInfo(i18n.T("output.no_state_found"))
		return nil
	}

	// Load state
	logging.DebugWithContext(ctx, "Loading project state", "workspace", wsRoot)
	stateDeps, err := uc.deps.StateManager.Load(ws)
	if err != nil {
		logging.ErrorWithContext(ctx, "Failed to load project state", "workspace", wsRoot, "error", err.Error())
		return errors.New(
			errors.ErrCodeStateLoadError,
			i18n.T("error.state_load"),
		).WithSuggestion(
			i18n.T("error.state_load_suggestion"),
		).WithContext("workspace", wsRoot).WithError(err)
	}
	logging.InfoWithContext(ctx, "Project state loaded", "project", stateDeps.Project.Name, "services_count", len(stateDeps.Services))

	composePath := uc.deps.Workspace.GetComposePath(ws)

	// Project-level down (default) vs full workspace down (--all)
	if !opts.All {
		done, err := uc.stopProjectServices(ctx, opts, composePath, projectName, workspaceName)
		if done {
			return err
		}
	}

	// Full workspace down
	output.PrintInfo(i18n.T("output.stopping_services"))
	if err := uc.deps.DockerRunner.DownWithContext(ctx, composePath); err != nil {
		return errors.New(
			errors.ErrCodeDockerNotRunning,
			i18n.T("error.docker_down_failed"),
		).WithSuggestion(
			i18n.T("error.docker_down_suggestion"),
		).WithContext("compose_file", composePath).WithError(err)
	}

	remainingNetworkProjects, isInUse := uc.handleNetworkAndVolumes(ctx, stateDeps, ws, projectName, workspaceName)

	// Remove state file
	statePath := uc.deps.Workspace.GetStatePath(ws)
	if err := os.Remove(statePath); err != nil && !os.IsNotExist(err) {
		return errors.New(
			errors.ErrCodeStateSaveError,
			i18n.T("error.state_remove"),
		).WithSuggestion(
			i18n.T("error.state_remove_suggestion"),
		).WithContext("state_path", statePath).WithError(err)
	}

	if err := uc.deps.StateManager.RemoveProject(projectName); err != nil {
		logging.Warn("failed to update global state", "error", err)
	}

	uc.handleProjectComposeDown(ctx, stateDeps, opts)
	uc.executeProjectDownCommand(ctx, stateDeps, ws, opts, workspaceName)
	uc.stopProxy(ctx, opts)
	uc.cleanLocalState(ctx, opts)

	output.PrintSuccess(i18n.T("output.project_stopped", stateDeps.Project.Name))

	networkName := stateDeps.Network.GetName()
	if remainingNetworkProjects > 0 {
		output.PrintInfo(i18n.T("output.network_in_use", networkName, remainingNetworkProjects))
	} else if isInUse {
		output.PrintInfo(i18n.T("output.network_in_use_containers", networkName))
	}

	uc.cleanupDockerResources(ctx)

	logging.InfoWithContext(ctx, "Project stopped successfully", "project", projectName)
	return nil
}

// resolveProject determines project name and workspace name from options.
func (uc *DownUseCase) resolveProject(ctx context.Context, opts DownOptions) (string, string, error) {
	projectName := opts.ProjectName
	var workspaceName string

	if projectName == "" {
		logging.DebugWithContext(ctx, "Project name not provided, loading from config", "config_path", opts.ConfigPath)
		deps, _, _ := uc.deps.ConfigLoader.LoadDeps(opts.ConfigPath)
		if deps != nil {
			return deps.Project.Name, deps.GetWorkspaceName(), nil
		}
		logging.ErrorWithContext(ctx, "Could not determine project name")
		return "", "", errors.New(
			errors.ErrCodeInvalidConfig,
			i18n.T("error.no_project"),
		).WithSuggestion(i18n.T("error.no_project_suggestion"))
	}

	deps, _, _ := uc.deps.ConfigLoader.LoadDeps(opts.ConfigPath)
	if deps != nil && deps.Project.Name == projectName {
		workspaceName = deps.GetWorkspaceName()
	} else {
		workspaceName = projectName
	}
	return projectName, workspaceName, nil
}

// resolveWorkspace resolves and validates the workspace.
func (uc *DownUseCase) resolveWorkspace(ctx context.Context, workspaceName, projectName string) (*interfaces.Workspace, string, error) {
	ws, err := uc.deps.Workspace.Resolve(workspaceName)
	if err != nil {
		logging.ErrorWithContext(ctx, "Failed to resolve workspace", "project", projectName, "error", err.Error())
		return nil, "", errors.New(
			errors.ErrCodeWorkspaceError,
			i18n.T("error.workspace_resolve"),
		).WithSuggestion(
			i18n.T("error.workspace_resolve_suggestion"),
		).WithContext("project", projectName).WithError(err)
	}
	wsRoot := uc.deps.Workspace.GetRoot(ws)
	logging.InfoWithContext(ctx, "Workspace resolved", "workspace", wsRoot)
	return ws, wsRoot, nil
}

// acquireLock acquires the workspace lock.
func (uc *DownUseCase) acquireLock(ctx context.Context, ws *interfaces.Workspace, wsRoot string) (interfaces.Lock, error) {
	logging.DebugWithContext(ctx, "Acquiring lock", "workspace", wsRoot)
	lockInstance, err := uc.deps.LockManager.Acquire(ws)
	if err != nil {
		logging.ErrorWithContext(ctx, "Failed to acquire lock", "workspace", wsRoot, "error", err.Error())
		return nil, errors.New(
			errors.ErrCodeLockError,
			i18n.T("error.lock_failed"),
		).WithSuggestion(
			i18n.T("error.lock_suggestion"),
		).WithContext("workspace", wsRoot).WithError(err)
	}
	logging.DebugWithContext(ctx, "Lock acquired successfully")
	return lockInstance, nil
}

// stopProjectServices stops only the current project's services (non-All mode).
// Returns (true, nil) if the operation is complete and Execute should return.
// Returns (false, nil) if caller should fall through to full workspace down.
func (uc *DownUseCase) stopProjectServices(
	ctx context.Context,
	opts DownOptions,
	composePath, projectName, workspaceName string,
) (bool, error) {
	currentDeps, _, _ := uc.deps.ConfigLoader.LoadDeps(opts.ConfigPath)
	if currentDeps == nil {
		output.PrintWarning(i18n.T("output.config_load_fallback"))
		return false, nil
	}

	servicesToStop := make(map[string]bool)
	for name := range currentDeps.Services {
		servicesToStop[name] = true
	}

	canPruneInfra := false
	if opts.PruneShared {
		globalState, err := uc.deps.StateManager.LoadGlobalState()
		if err == nil {
			activeInWorkspace := 0
			for name, p := range globalState.Projects {
				if name == projectName {
					continue
				}
				if p.Workspace == workspaceName {
					activeInWorkspace++
				}
			}
			canPruneInfra = activeInWorkspace == 0
		}
	}
	if canPruneInfra {
		for name := range currentDeps.Infra {
			servicesToStop[name] = true
		}
	}

	if len(servicesToStop) == 0 {
		output.PrintInfo(i18n.T("output.no_services_to_stop"))
	} else {
		output.PrintInfo(i18n.T("output.stopping_project_services"))
		for name := range servicesToStop {
			if err := uc.deps.DockerRunner.StopServiceWithContext(ctx, composePath, name); err != nil {
				logging.WarnWithContext(ctx, "Failed to stop service during project down", "service", name, "error", err.Error())
			}
		}
	}

	if err := uc.deps.StateManager.RemoveProject(projectName); err != nil {
		logging.Warn("failed to update global state", "error", err)
	}

	if canPruneInfra {
		output.PrintInfo(i18n.T("output.infra_pruned"))
	} else if opts.PruneShared {
		output.PrintInfo(i18n.T("output.infra_kept"))
	}
	output.PrintSuccess(i18n.T("output.project_stopped", projectName))
	return true, nil
}
