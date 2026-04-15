package app

import (
	"context"
	"slices"

	"raioz/internal/errors"
	"raioz/internal/i18n"
	"raioz/internal/output"
)

// ExecOptions contains options for the Exec use case
type ExecOptions struct {
	ConfigPath  string
	ProjectName string
	Service     string
	Command     []string
	Interactive bool
}

// ExecUseCase handles the "exec" use case
type ExecUseCase struct {
	deps *Dependencies
}

// NewExecUseCase creates a new ExecUseCase
func NewExecUseCase(deps *Dependencies) *ExecUseCase {
	return &ExecUseCase{deps: deps}
}

// Execute runs a command inside a running container
func (uc *ExecUseCase) Execute(ctx context.Context, opts ExecOptions) error {
	if ctx == nil {
		ctx = context.Background()
	}

	// Try YAML mode first
	if proj := ResolveYAMLProject(uc.deps, opts.ConfigPath); proj != nil {
		return ExecYAML(ctx, proj, opts.Service, opts.Command, opts.Interactive)
	}

	// Legacy: resolve project
	projectName := opts.ProjectName
	var workspaceName string
	if projectName == "" {
		deps, warnings, _ := uc.deps.ConfigLoader.LoadDeps(opts.ConfigPath)
		for _, w := range warnings {
			output.PrintWarning(w)
		}
		if deps != nil {
			projectName = deps.Project.Name
			workspaceName = deps.GetWorkspaceName()
		} else {
			return errors.New(
				errors.ErrCodeInvalidConfig,
				i18n.T("error.no_project"),
			).WithSuggestion(i18n.T("error.no_project_suggestion"))
		}
	} else {
		deps, warnings, _ := uc.deps.ConfigLoader.LoadDeps(opts.ConfigPath)
		for _, w := range warnings {
			output.PrintWarning(w)
		}
		if deps != nil && deps.Project.Name == projectName {
			workspaceName = deps.GetWorkspaceName()
		} else {
			workspaceName = projectName
		}
	}

	ws, err := uc.deps.Workspace.Resolve(workspaceName)
	if err != nil {
		return errors.New(
			errors.ErrCodeWorkspaceError,
			i18n.T("error.workspace_resolve"),
		).WithContext("project", projectName).WithError(err)
	}

	if !uc.deps.StateManager.Exists(ws) {
		return errors.New(
			errors.ErrCodeWorkspaceError,
			i18n.T("error.exec_not_running"),
		).WithContext("project", projectName)
	}

	// Load state to get ProjectComposePath
	stateDeps, err := uc.deps.StateManager.Load(ws)
	if err != nil {
		return errors.New(
			errors.ErrCodeStateLoadError,
			i18n.T("error.state_load_previous"),
		).WithError(err)
	}

	composePath := uc.deps.Workspace.GetComposePath(ws)

	// Check if service is a host service (source.command)
	hostProcesses, _ := uc.deps.HostRunner.LoadProcessesState(ws)
	if hostProcesses != nil {
		if _, isHost := hostProcesses[opts.Service]; isHost {
			return errors.New(
				errors.ErrCodeInvalidField,
				i18n.T("error.exec_host_service", opts.Service, opts.Service),
			).WithContext("service", opts.Service)
		}
	}

	// Search service in generated compose
	available, err := uc.deps.DockerRunner.GetAvailableServicesWithContext(ctx, composePath)
	if err != nil {
		return errors.New(
			errors.ErrCodeDockerNotRunning,
			i18n.T("error.exec_services_failed"),
		).WithError(err)
	}

	if slices.Contains(available, opts.Service) {
		return uc.execInCompose(ctx, composePath, opts)
	}

	// Search service in project compose
	if stateDeps != nil && stateDeps.ProjectComposePath != "" {
		projectAvailable, err := uc.deps.DockerRunner.GetAvailableServicesWithContext(ctx, stateDeps.ProjectComposePath)
		if err == nil && slices.Contains(projectAvailable, opts.Service) {
			return uc.execInCompose(ctx, stateDeps.ProjectComposePath, opts)
		}
	}

	return errors.New(
		errors.ErrCodeInvalidField,
		i18n.T("error.exec_service_not_found", opts.Service),
	).WithContext("service", opts.Service)
}

// execInCompose executes a command in a service via docker compose
func (uc *ExecUseCase) execInCompose(ctx context.Context, composePath string, opts ExecOptions) error {
	command := opts.Command
	if len(command) == 0 {
		command = []string{"sh"}
	}
	return uc.deps.DockerRunner.ExecInService(ctx, composePath, opts.Service, command, opts.Interactive)
}
