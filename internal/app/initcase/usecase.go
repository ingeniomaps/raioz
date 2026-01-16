package initcase

import (
	"context"
)

// Options contains options for the Init use case
type Options struct {
	OutputPath string
}

// UseCase handles the "init" use case - initializing a new .raioz.json configuration file
type UseCase struct {
}

// NewUseCase creates a new InitUseCase
func NewUseCase() *UseCase {
	return &UseCase{}
}

// Execute executes the init use case
func (uc *UseCase) Execute(ctx context.Context, opts Options) error {
	// Show welcome message
	uc.showWelcomeMessage()

	// Check if file already exists and ask for confirmation
	shouldContinue, err := uc.checkFileExists(opts.OutputPath)
	if err != nil {
		return err
	}
	if !shouldContinue {
		return nil
	}

	// Prompt user for project information
	projectName, networkName, err := uc.promptProjectInfo()
	if err != nil {
		return err
	}

	// Create and validate configuration
	deps, err := uc.createConfig(projectName, networkName)
	if err != nil {
		return err
	}

	// Write configuration file
	if err := uc.writeConfigFile(opts.OutputPath, deps); err != nil {
		return err
	}

	// Show success message
	uc.showSuccessMessage(opts.OutputPath)

	return nil
}
