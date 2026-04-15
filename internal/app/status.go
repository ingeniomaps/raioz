package app

import (
	"context"
	"fmt"
	"os"
	"strings"

	"raioz/internal/config"
	"raioz/internal/domain/interfaces"
	"raioz/internal/errors"
	"raioz/internal/host"
	"raioz/internal/i18n"
	"raioz/internal/output"
)

// StatusOptions contains options for the Status use case
type StatusOptions struct {
	ProjectName string
	ConfigPath  string
	JSON        bool
}

// StatusUseCase handles the "status" use case - showing project status
type StatusUseCase struct {
	deps *Dependencies
}

// NewStatusUseCase creates a new StatusUseCase with injected dependencies
func NewStatusUseCase(deps *Dependencies) *StatusUseCase {
	return &StatusUseCase{
		deps: deps,
	}
}

// Execute executes the status use case
func (uc *StatusUseCase) Execute(ctx context.Context, opts StatusOptions) error {
	if ctx == nil {
		ctx = context.Background()
	}

	// Try YAML mode first
	if proj := ResolveYAMLProject(uc.deps, opts.ConfigPath); proj != nil {
		return uc.StatusYAML(ctx, proj)
	}

	var ws *interfaces.Workspace
	var err error

	// Legacy: try to determine project name and workspace
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
			).WithSuggestion(
				i18n.T("error.no_project_suggestion"),
			)
		}
	} else {
		// If project name comes from CLI, load config to get workspace name
		deps, warnings, _ := uc.deps.ConfigLoader.LoadDeps(opts.ConfigPath)
		for _, w := range warnings {
			output.PrintWarning(w)
		}
		if deps != nil && deps.Project.Name == projectName {
			workspaceName = deps.GetWorkspaceName()
		} else {
			// Fallback: use project name as workspace (backward compatibility)
			workspaceName = projectName
		}
	}

	// Resolve workspace using workspace name
	ws, err = uc.deps.Workspace.Resolve(workspaceName)
	if err != nil {
		return errors.New(
			errors.ErrCodeWorkspaceError,
			i18n.T("error.workspace_resolve"),
		).WithSuggestion(
			i18n.T("error.workspace_resolve_suggestion"),
		).WithContext("project", projectName).WithError(err)
	}

	// Check if state exists
	if !uc.deps.StateManager.Exists(ws) {
		if opts.JSON {
			fmt.Println("{}")
		} else {
			fmt.Println(i18n.T("output.project_not_running_status"))
		}
		return nil
	}

	// Load original .raioz.json to check for disabled services
	originalDeps, _, err := uc.deps.ConfigLoader.LoadDeps(opts.ConfigPath)
	if err != nil {
		return errors.New(
			errors.ErrCodeInvalidConfig,
			i18n.T("error.config_load"),
		).WithSuggestion(
			i18n.T("error.config_load_suggestion"),
		).WithError(err)
	}

	// Load state
	stateDeps, err := uc.deps.StateManager.Load(ws)
	if err != nil {
		return errors.New(
			errors.ErrCodeStateLoadError,
			i18n.T("error.state_load"),
		).WithSuggestion(
			i18n.T("error.state_load_suggestion_recreate"),
		).WithContext("workspace", uc.deps.Workspace.GetRoot(ws)).WithError(err)
	}

	composePath := uc.deps.Workspace.GetComposePath(ws)

	// Load host processes state to check host services
	hostProcesses, err := uc.deps.HostRunner.LoadProcessesState(ws)
	if err != nil {
		// Log but continue - host processes state is optional
		hostProcesses = make(map[string]*host.ProcessInfo)
	}

	// Collect all service names from state (running services)
	serviceNamesMap := make(map[string]bool)
	for name := range stateDeps.Services {
		serviceNamesMap[name] = true
	}
	for name := range stateDeps.Infra {
		serviceNamesMap[name] = true
	}

	// Also include host services from host processes state
	for name := range hostProcesses {
		serviceNamesMap[name] = true
	}

	// Convert map to slice
	var serviceNames []string
	for name := range serviceNamesMap {
		serviceNames = append(serviceNames, name)
	}

	// Find disabled services (in original deps but not in state)
	var disabledServices []string
	envVars := make(map[string]string)
	for _, key := range os.Environ() {
		pair := strings.SplitN(key, "=", 2)
		if len(pair) == 2 {
			envVars[pair[0]] = pair[1]
		}
	}
	for name, svc := range originalDeps.Services {
		// Check if service is disabled
		if !uc.deps.ConfigLoader.IsServiceEnabled(svc, "", envVars) {
			disabledServices = append(disabledServices, name)
		}
	}

	// Get detailed service info (for Docker services from generated compose)
	servicesInfo, err := uc.deps.DockerRunner.GetServicesInfoWithContext(
		ctx,
		composePath,
		serviceNames,
		stateDeps.Project.Name,
		stateDeps.Services,
		ws,
	)
	if err != nil {
		return errors.New(errors.ErrCodeDockerNotRunning, i18n.T("error.status_services_info")).WithError(err)
	}

	// If project has its own docker-compose.yml, include services from it
	if stateDeps.ProjectComposePath != "" {
		projectComposeServices, err := uc.deps.DockerRunner.GetAvailableServicesWithContext(ctx, stateDeps.ProjectComposePath)
		if err == nil && len(projectComposeServices) > 0 {
			// Get service info from project compose
			projectServicesInfo, err := uc.deps.DockerRunner.GetServicesInfoWithContext(
				ctx,
				stateDeps.ProjectComposePath,
				projectComposeServices,
				stateDeps.Project.Name,
				make(map[string]config.Service), // No service configs for project compose services
				ws,
			)
			if err == nil {
				// Merge project compose services into servicesInfo (use original names)
				for name, info := range projectServicesInfo {
					servicesInfo[name] = info
				}
			}
		}
	}

	// Check host services status
	for _, name := range serviceNames {
		// Check if this is a host service (not in docker-compose.generated.yml)
		svc, exists := originalDeps.Services[name]
		if !exists {
			continue
		}

		// Skip if it's a Docker service (has docker config and no commands)
		if svc.Docker != nil && (svc.Commands == nil || (svc.Commands.Up == "" && svc.Source.Command == "")) {
			continue // Already handled by DockerRunner above
		}

		// This is a host service - check its status
		info := uc.getHostServiceInfo(ctx, ws, name, svc, originalDeps, hostProcesses)
		if info != nil {
			servicesInfo[name] = info
		}
	}

	// Get active workspace for output
	activeWorkspace, err := uc.deps.Workspace.GetActiveWorkspace()
	if err != nil {
		activeWorkspace = "" // Ignore error, just use empty
	}

	// Output format
	if opts.JSON {
		return uc.outputJSON(servicesInfo, disabledServices, stateDeps, activeWorkspace)
	}

	// Output in human-readable format
	return uc.outputHumanReadable(servicesInfo, disabledServices, stateDeps, activeWorkspace)
}
