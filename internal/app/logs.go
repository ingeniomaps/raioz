package app

import (
	"context"

	"raioz/internal/domain/interfaces"
	"raioz/internal/errors"
	"raioz/internal/i18n"
)

// LogsOptions contains options for the Logs use case
type LogsOptions struct {
	ConfigPath  string
	ProjectName string
	Follow      bool
	Tail        int
	All         bool
	Services    []string
}

// LogsUseCase handles the "logs" use case
type LogsUseCase struct {
	deps *Dependencies
}

// NewLogsUseCase creates a new LogsUseCase with injected dependencies
func NewLogsUseCase(deps *Dependencies) *LogsUseCase {
	return &LogsUseCase{deps: deps}
}

// Execute executes the logs use case
func (uc *LogsUseCase) Execute(ctx context.Context, opts LogsOptions) error {
	projectName, workspaceName, err := uc.resolveProject(opts)
	if err != nil {
		return err
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
			i18n.T("error.logs_not_running"),
		).WithContext("project", projectName)
	}

	stateDeps, err := uc.deps.StateManager.Load(ws)
	if err != nil {
		return errors.New(
			errors.ErrCodeStateLoadError,
			i18n.T("error.state_load_previous"),
		).WithError(err)
	}

	composePath := uc.deps.Workspace.GetComposePath(ws)

	var projectComposePath string
	if stateDeps != nil {
		projectComposePath = stateDeps.ProjectComposePath
	}
	services, projectComposeServices, err := uc.resolveServices(ctx, opts, composePath, projectComposePath)
	if err != nil {
		return err
	}

	// View logs from generated compose
	if len(services) > 0 {
		logsOpts := interfaces.LogsOptions{
			Follow:   opts.Follow,
			Tail:     opts.Tail,
			Services: services,
		}
		if err := uc.deps.DockerRunner.ViewLogsWithContext(ctx, composePath, logsOpts); err != nil {
			return errors.New(
				errors.ErrCodeDockerNotRunning,
				i18n.T("error.logs_view_failed"),
			).WithError(err)
		}
	}

	// View logs from project compose if it exists
	if len(projectComposeServices) > 0 && stateDeps != nil && stateDeps.ProjectComposePath != "" {
		logsOpts := interfaces.LogsOptions{
			Follow:   opts.Follow,
			Tail:     opts.Tail,
			Services: projectComposeServices,
		}
		if err := uc.deps.DockerRunner.ViewLogsWithContext(ctx, stateDeps.ProjectComposePath, logsOpts); err != nil {
			return errors.New(
				errors.ErrCodeDockerNotRunning,
				i18n.T("error.logs_view_failed"),
			).WithError(err)
		}
	}

	return nil
}

func (uc *LogsUseCase) resolveProject(opts LogsOptions) (string, string, error) {
	projectName := opts.ProjectName
	var workspaceName string

	if projectName == "" {
		deps, _, _ := uc.deps.ConfigLoader.LoadDeps(opts.ConfigPath)
		if deps != nil {
			return deps.Project.Name, deps.GetWorkspaceName(), nil
		}
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

func (uc *LogsUseCase) resolveServices(
	ctx context.Context,
	opts LogsOptions,
	composePath string,
	projectComposePath string,
) ([]string, []string, error) {
	if opts.All || len(opts.Services) == 0 {
		allServices, err := uc.deps.DockerRunner.GetAvailableServicesWithContext(ctx, composePath)
		if err != nil {
			return nil, nil, errors.New(
				errors.ErrCodeDockerNotRunning,
				i18n.T("error.exec_services_failed"),
			).WithError(err)
		}

		var projectComposeServices []string
		if projectComposePath != "" {
			projectServices, err := uc.deps.DockerRunner.GetAvailableServicesWithContext(ctx, projectComposePath)
			if err == nil {
				projectComposeServices = projectServices
			}
		}

		return allServices, projectComposeServices, nil
	}

	// Services specified as arguments
	generatedServices, _ := uc.deps.DockerRunner.GetAvailableServicesWithContext(ctx, composePath)

	if projectComposePath == "" {
		return opts.Services, nil, nil
	}

	projectServices, err := uc.deps.DockerRunner.GetAvailableServicesWithContext(ctx, projectComposePath)
	if err != nil {
		return opts.Services, nil, nil
	}

	var services, projectComposeServices []string
	for _, arg := range opts.Services {
		foundInGenerated := false
		for _, genSvc := range generatedServices {
			if arg == genSvc {
				services = append(services, arg)
				foundInGenerated = true
				break
			}
		}
		if !foundInGenerated {
			for _, projSvc := range projectServices {
				if arg == projSvc {
					projectComposeServices = append(projectComposeServices, arg)
					break
				}
			}
		}
	}

	return services, projectComposeServices, nil
}
