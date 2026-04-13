package app

import (
	"context"
	"fmt"
	"io"
	"os"

	"raioz/internal/config"
	"raioz/internal/errors"
	"raioz/internal/i18n"
)

// RestartOptions contains options for the Restart use case
type RestartOptions struct {
	ConfigPath    string
	ProjectName   string
	All           bool
	IncludeInfra  bool
	ForceRecreate bool
	Services      []string
}

// RestartUseCase handles the "restart" use case
type RestartUseCase struct {
	deps *Dependencies
	Out  io.Writer
}

// NewRestartUseCase creates a new RestartUseCase
func NewRestartUseCase(deps *Dependencies) *RestartUseCase {
	return &RestartUseCase{deps: deps, Out: os.Stdout}
}

// Execute restarts services
func (uc *RestartUseCase) Execute(ctx context.Context, opts RestartOptions) error {
	if ctx == nil {
		ctx = context.Background()
	}

	// Try YAML mode first
	if proj := ResolveYAMLProject(uc.deps, opts.ConfigPath); proj != nil {
		return RestartYAML(ctx, proj, opts.Services)
	}

	w := uc.Out

	// Legacy: resolve project
	projectName := opts.ProjectName
	var workspaceName string
	if projectName == "" {
		deps, _, _ := uc.deps.ConfigLoader.LoadDeps(opts.ConfigPath)
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
		deps, _, _ := uc.deps.ConfigLoader.LoadDeps(opts.ConfigPath)
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
			i18n.T("error.restart_not_running"),
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

	// Check for host services in specific service requests
	if len(opts.Services) > 0 {
		hostProcesses, _ := uc.deps.HostRunner.LoadProcessesState(ws)
		if hostProcesses != nil {
			for _, svc := range opts.Services {
				if _, isHost := hostProcesses[svc]; isHost {
					return errors.New(
						errors.ErrCodeInvalidField,
						i18n.T("error.restart_host_service", svc),
					).WithContext("service", svc)
				}
			}
		}
	}

	servicesToRestart, err := uc.resolveRestartServices(ctx, opts, stateDeps, composePath)
	if err != nil {
		return err
	}

	// Also check ProjectComposePath for services not found in generated compose
	if len(servicesToRestart) == 0 && len(opts.Services) > 0 && stateDeps != nil && stateDeps.ProjectComposePath != "" {
		servicesToRestart, err = uc.resolveProjectComposeServices(ctx, opts, stateDeps.ProjectComposePath)
		if err != nil {
			return err
		}
		if len(servicesToRestart) > 0 {
			return uc.doRestart(ctx, w, stateDeps.ProjectComposePath, servicesToRestart, opts.ForceRecreate)
		}
	}

	if len(servicesToRestart) == 0 {
		return errors.New(
			errors.ErrCodeInvalidField,
			i18n.T("error.restart_no_services"),
		)
	}

	return uc.doRestart(ctx, w, composePath, servicesToRestart, opts.ForceRecreate)
}

// doRestart performs the actual restart operation
func (uc *RestartUseCase) doRestart(
	ctx context.Context, w io.Writer, composePath string,
	services []string, forceRecreate bool,
) error {
	fmt.Fprintf(w, "⏳ %s\n", i18n.T("output.restarting_services"))

	var err error
	if forceRecreate {
		err = uc.deps.DockerRunner.ForceRecreateServicesWithContext(ctx, composePath, services)
	} else {
		err = uc.deps.DockerRunner.RestartServicesWithContext(ctx, composePath, services)
	}

	if err != nil {
		return errors.New(
			errors.ErrCodeDockerNotRunning,
			i18n.T("error.restart_failed"),
		).WithSuggestion(i18n.T("error.restart_suggestion")).WithError(err)
	}

	fmt.Fprintf(w, "✔ %s\n", i18n.T("output.services_restarted", len(services), services))
	return nil
}

func (uc *RestartUseCase) resolveRestartServices(
	ctx context.Context,
	opts RestartOptions,
	stateDeps *config.Deps,
	composePath string,
) ([]string, error) {
	available, err := uc.deps.DockerRunner.GetAvailableServicesWithContext(ctx, composePath)
	if err != nil {
		return nil, errors.New(
			errors.ErrCodeDockerNotRunning,
			i18n.T("error.exec_services_failed"),
		).WithError(err)
	}
	availableSet := make(map[string]bool)
	for _, s := range available {
		availableSet[s] = true
	}

	// Specific services requested
	if len(opts.Services) > 0 {
		var found []string
		for _, svc := range opts.Services {
			if availableSet[svc] {
				found = append(found, svc)
			}
		}
		// If none found in generated compose, return empty (caller will check ProjectComposePath)
		return found, nil
	}

	if !opts.All {
		return nil, nil
	}

	// --all: filter infra unless --include-infra
	if opts.IncludeInfra {
		return available, nil
	}

	// Build infra set from state
	infraSet := make(map[string]bool)
	if stateDeps != nil {
		for name := range stateDeps.Infra {
			infraSet[name] = true
		}
	}

	var servicesOnly []string
	for _, svc := range available {
		if !infraSet[svc] {
			servicesOnly = append(servicesOnly, svc)
		}
	}
	return servicesOnly, nil
}

// resolveProjectComposeServices resolves services from ProjectComposePath
func (uc *RestartUseCase) resolveProjectComposeServices(
	ctx context.Context,
	opts RestartOptions,
	projectComposePath string,
) ([]string, error) {
	available, err := uc.deps.DockerRunner.GetAvailableServicesWithContext(ctx, projectComposePath)
	if err != nil {
		return nil, nil // project compose may not be accessible
	}

	availableSet := make(map[string]bool)
	for _, s := range available {
		availableSet[s] = true
	}

	var found []string
	for _, svc := range opts.Services {
		if availableSet[svc] {
			found = append(found, svc)
		}
	}

	if len(found) == 0 && len(opts.Services) > 0 {
		return nil, errors.New(
			errors.ErrCodeInvalidField,
			i18n.T("error.restart_service_not_found", opts.Services[0]),
		).WithContext("service", opts.Services[0])
	}

	return found, nil
}
