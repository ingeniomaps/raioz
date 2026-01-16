package app

import (
	"context"

	initcase "raioz/internal/app/initcase"
)

// InitOptions contains options for the Init use case
type InitOptions struct {
	OutputPath string
}

// InitUseCase handles the "init" use case - initializing a new .raioz.json configuration file
type InitUseCase struct {
	useCase *initcase.UseCase
}

// NewInitUseCase creates a new InitUseCase
func NewInitUseCase(deps *Dependencies) *InitUseCase {
	return &InitUseCase{
		useCase: initcase.NewUseCase(),
	}
}

// Execute executes the init use case
func (uc *InitUseCase) Execute(ctx context.Context, opts InitOptions) error {
	options := initcase.Options{
		OutputPath: opts.OutputPath,
	}
	return uc.useCase.Execute(ctx, options)
}
