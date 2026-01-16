package app

import (
	"context"

	checkcase "raioz/internal/app/checkcase"
)

// CheckOptions contains options for the Check use case
type CheckOptions struct {
	ProjectName string
	ConfigPath  string
}

// CheckUseCase handles the "check" use case - checking alignment between config and state
type CheckUseCase struct {
	useCase *checkcase.UseCase
}

// NewCheckUseCase creates a new CheckUseCase with injected dependencies
func NewCheckUseCase(deps *Dependencies) *CheckUseCase {
	return &CheckUseCase{
		useCase: checkcase.NewUseCase(&checkcase.Dependencies{
			ConfigLoader: deps.ConfigLoader,
			Workspace:    deps.Workspace,
			StateManager: deps.StateManager,
		}),
	}
}

// Execute executes the check use case
func (uc *CheckUseCase) Execute(ctx context.Context, opts CheckOptions) error {
	options := checkcase.Options{
		ProjectName: opts.ProjectName,
		ConfigPath:  opts.ConfigPath,
	}
	return uc.useCase.Execute(ctx, options)
}
