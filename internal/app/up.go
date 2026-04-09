package app

import (
	"context"

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
}

// UpUseCase handles the "up" use case - starting a project
type UpUseCase struct {
	useCase *upcase.UseCase
}

// NewUpUseCase creates a new UpUseCase with injected dependencies
func NewUpUseCase(deps *Dependencies) *UpUseCase {
	return &UpUseCase{
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
	options := upcase.Options{
		ConfigPath:   opts.ConfigPath,
		Profile:      opts.Profile,
		ForceReclone: opts.ForceReclone,
		DryRun:       opts.DryRun,
		Only:         opts.Only,
		Host:         opts.Host,
	}
	return uc.useCase.Execute(ctx, options)
}
