package app

import (
	"context"

	"raioz/internal/i18n"
	"raioz/internal/logging"
	"raioz/internal/output"

	upcase "raioz/internal/app/upcase"
)

// UpOptions contains options for the Up use case
type UpOptions struct {
	ConfigPath   string
	Profile      string
	ForceReclone bool
	DryRun       bool
	Only         []string
	Host         string // Bind address for shared dev server (e.g., "0.0.0.0")
	Attach       bool   // Stay attached and stream logs without file watching
	Exclusive    bool   // Stop other projects before starting this one
}

// UpUseCase handles the "up" use case - starting a project
type UpUseCase struct {
	deps    *Dependencies
	useCase *upcase.UseCase
}

// NewUpUseCase creates a new UpUseCase with injected dependencies
func NewUpUseCase(deps *Dependencies) *UpUseCase {
	return &UpUseCase{
		deps: deps,
		useCase: upcase.NewUseCase(&upcase.Dependencies{
			ConfigLoader:     deps.ConfigLoader,
			Validator:        deps.Validator,
			DockerRunner:     deps.DockerRunner,
			GitRepository:    deps.GitRepository,
			Workspace:        deps.Workspace,
			StateManager:     deps.StateManager,
			LockManager:      deps.LockManager,
			HostRunner:       deps.HostRunner,
			EnvManager:       deps.EnvManager,
			ProxyManager:     deps.ProxyManager,
			DiscoveryManager: deps.DiscoveryManager,
		}),
	}
}

// Execute executes the up use case
func (uc *UpUseCase) Execute(ctx context.Context, opts UpOptions) error {
	if opts.Exclusive {
		uc.stopOtherProjects(ctx, opts.ConfigPath)
	}

	options := upcase.Options{
		ConfigPath:   opts.ConfigPath,
		Profile:      opts.Profile,
		ForceReclone: opts.ForceReclone,
		DryRun:       opts.DryRun,
		Only:         opts.Only,
		Host:         opts.Host,
		Attach:       opts.Attach,
	}
	return uc.useCase.Execute(ctx, options)
}

// stopOtherProjects stops all running projects except the current one.
func (uc *UpUseCase) stopOtherProjects(ctx context.Context, configPath string) {
	globalState, err := uc.deps.StateManager.LoadGlobalState()
	if err != nil {
		return
	}

	if len(globalState.ActiveProjects) == 0 {
		return
	}

	// Determine current project name from config
	currentProject := ""
	if configPath != "" {
		deps, _, loadErr := uc.deps.ConfigLoader.LoadDeps(configPath)
		if loadErr == nil && deps != nil {
			currentProject = deps.Project.Name
		}
	}

	stopped := 0
	for _, name := range globalState.ActiveProjects {
		if name == currentProject {
			continue
		}

		logging.InfoWithContext(ctx,
			i18n.T("up.exclusive_stopping"),
			"project", name,
		)

		downUC := NewDownUseCase(uc.deps)
		if err := downUC.Execute(ctx, DownOptions{ProjectName: name}); err != nil {
			logging.WarnWithContext(ctx,
				i18n.T("up.exclusive_stop_failed"),
				"project", name, "error", err.Error(),
			)
			continue
		}
		stopped++
	}

	if stopped > 0 {
		output.PrintSuccess(i18n.T("up.exclusive_stopped", stopped))
	}
}
