package app

import (
	"context"

	"raioz/internal/i18n"
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
	Attach       bool   // Stay foreground streaming logs (no file watching)
	Watch        bool   // Stay foreground file-watching services with watch: true
	Exclusive    bool   // Stop other projects before starting this one
	Yes          bool   // Approve up front what Exclusive would stop
	// RouterOff forces the bundled Caddy to start even when
	// RAIOZ_ROUTER_ACTIVE=1 is inherited from the shell.
	RouterOff bool
	// AuditSiblings runs ADR-036 hygiene gates on every sibling
	// dependency yaml before spawn. Opt-in.
	AuditSiblings bool
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
		proceed, err := uc.stopOtherProjects(ctx, opts.ConfigPath, opts.Yes)
		if err != nil || !proceed {
			return err
		}
	}

	options := upcase.Options{
		ConfigPath:    opts.ConfigPath,
		Profile:       opts.Profile,
		ForceReclone:  opts.ForceReclone,
		DryRun:        opts.DryRun,
		Only:          opts.Only,
		Host:          opts.Host,
		Attach:        opts.Attach,
		Watch:         opts.Watch,
		RouterOff:     opts.RouterOff,
		AuditSiblings: opts.AuditSiblings,
	}
	return uc.useCase.Execute(ctx, options)
}

// stopOtherProjects stops every other running project, once the user has
// approved the list. proceed is false when they declined: an exclusive up
// that could not clear the others does not start, rather than come up next
// to what it was asked to replace.
func (uc *UpUseCase) stopOtherProjects(
	ctx context.Context, configPath string, yes bool,
) (proceed bool, err error) {
	// Determine current project name from config
	currentProject := ""
	if configPath != "" {
		deps, _, loadErr := uc.deps.ConfigLoader.LoadDeps(configPath)
		if loadErr == nil && deps != nil {
			currentProject = deps.Project.Name
		}
	}

	declined := false
	approve := approveStopping(yes, reasonExclusive)
	stopped, err := DownAllOtherProjects(ctx, currentProject, func(names []string) (bool, error) {
		ok, approveErr := approve(names)
		declined = !ok && approveErr == nil
		return ok, approveErr
	})
	if err != nil {
		return false, err
	}
	if declined {
		return false, nil
	}
	if len(stopped) > 0 {
		output.PrintSuccess(i18n.T("up.exclusive_stopped", len(stopped)))
	}
	return true, nil
}
