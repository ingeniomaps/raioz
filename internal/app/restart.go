package app

import (
	"context"
	"fmt"
	"io"
	"os"

	"raioz/internal/domain/models"
	"raioz/internal/errors"
	"raioz/internal/i18n"
	"raioz/internal/output"
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
		// Restart mutates `.raioz.state.json` (new PIDs after re-launch).
		// Acquire the workspace lock so a concurrent `raioz up --watch`
		// save-state can't race. Issue 038.
		release, err := acquireRestartLock(ctx, uc.deps, proj.Deps.Project.Name)
		if err != nil {
			return err
		}
		defer release()
		return uc.RestartYAML(ctx, proj, opts)
	}

	w := uc.Out

	// Legacy: resolve project. Keep the loaded deps in scope so the
	// later restart logic can read services/infra from it (post-ADR-011
	// the legacy snapshot is gone — current deps is the source of
	// truth).
	deps, warnings, _ := uc.deps.ConfigLoader.LoadDeps(opts.ConfigPath)
	for _, w := range warnings {
		output.PrintWarning(w)
	}
	projectName := opts.ProjectName
	var workspaceName string
	if projectName == "" {
		if deps == nil {
			return errors.New(
				errors.ErrCodeInvalidConfig,
				i18n.T("error.no_project"),
			).WithSuggestion(i18n.T("error.no_project_suggestion"))
		}
		projectName = deps.Project.Name
		workspaceName = deps.GetWorkspaceName()
	} else if deps != nil && deps.Project.Name == projectName {
		workspaceName = deps.GetWorkspaceName()
	} else {
		workspaceName = projectName
	}

	ws, err := uc.deps.Workspace.Resolve(workspaceName)
	if err != nil {
		return errors.New(
			errors.ErrCodeWorkspaceError,
			i18n.T("error.workspace_resolve"),
		).WithContext("project", projectName).WithError(err)
	}

	// ADR-011 Phase 2: docker labels for liveness, LocalState for the
	// project-level compose path. The current deps (above) replaces the
	// legacy snapshot for the services / infra map.
	active, err := uc.deps.DockerRunner.IsProjectActive(ctx, workspaceName, projectName)
	if err != nil {
		return errors.New(
			errors.ErrCodeStateLoadError,
			i18n.T("error.state_load_previous"),
		).WithError(err)
	}
	if !active {
		return errors.New(
			errors.ErrCodeWorkspaceError,
			i18n.T("error.restart_not_running"),
		).WithContext("project", projectName)
	}

	projectComposePath := loadProjectComposePathFromLocalState(opts.ConfigPath)
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

	servicesToRestart, err := uc.resolveRestartServices(ctx, opts, deps, composePath)
	if err != nil {
		return err
	}

	// Also check ProjectComposePath for services not found in generated compose
	if len(servicesToRestart) == 0 && len(opts.Services) > 0 && projectComposePath != "" {
		servicesToRestart, err = uc.resolveProjectComposeServices(ctx, opts, projectComposePath)
		if err != nil {
			return err
		}
		if len(servicesToRestart) > 0 {
			return uc.doRestart(ctx, w, projectComposePath, servicesToRestart, opts.ForceRecreate)
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
	stateDeps *models.Deps,
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
