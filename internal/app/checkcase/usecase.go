package checkcase

import (
	"context"

	"raioz/internal/domain/interfaces"
)

// Options contains options for the Check use case
type Options struct {
	ProjectName string
	ConfigPath  string
}

// Dependencies contains the dependencies needed by the check use case
type Dependencies struct {
	ConfigLoader interfaces.ConfigLoader
	Workspace    interfaces.WorkspaceManager
	StateManager interfaces.StateManager
}

// UseCase handles the "check" use case - checking alignment between config and state
type UseCase struct {
	deps *Dependencies
}

// NewUseCase creates a new CheckUseCase with injected dependencies
func NewUseCase(deps *Dependencies) *UseCase {
	return &UseCase{
		deps: deps,
	}
}

// Execute executes the check use case
func (uc *UseCase) Execute(ctx context.Context, opts Options) error {
	// Ensure context
	if ctx == nil {
		ctx = context.Background()
	}

	// Resolve workspace and determine project name
	_, ws, err := uc.resolveWorkspace(opts)
	if err != nil {
		return err
	}

	// Check if state exists
	if !uc.deps.StateManager.Exists(ws) {
		return uc.handleNoState()
	}

	// Load current config
	currentDeps, err := uc.loadConfig(opts.ConfigPath)
	if err != nil {
		return err
	}

	// Check alignment and display results
	return uc.checkAndDisplayAlignment(ws, currentDeps)
}
