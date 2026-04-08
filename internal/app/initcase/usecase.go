package initcase

import (
	"bufio"
	"context"
	"io"
	"os"

	"raioz/internal/config"
)

// Options contains options for the Init use case
type Options struct {
	OutputPath string
}

// UseCase handles the "init" use case
type UseCase struct {
	In     io.Reader
	Out    io.Writer
	reader *bufio.Reader
}

// NewUseCase creates a new InitUseCase with stdin/stdout defaults
func NewUseCase() *UseCase {
	return &UseCase{
		In:  os.Stdin,
		Out: os.Stdout,
	}
}

// Execute executes the init use case
func (uc *UseCase) Execute(ctx context.Context, opts Options) error {
	uc.reader = bufio.NewReader(uc.In)

	uc.showWelcomeMessage()

	shouldContinue, err := uc.checkFileExists(opts.OutputPath)
	if err != nil {
		return err
	}
	if !shouldContinue {
		return nil
	}

	projectName, networkName, err := uc.promptProjectInfo()
	if err != nil {
		return err
	}

	services, err := uc.promptServices()
	if err != nil {
		return err
	}

	var infra map[string]config.InfraEntry
	infra, err = uc.promptInfra()
	if err != nil {
		return err
	}

	deps, err := uc.createConfig(projectName, networkName, services, infra)
	if err != nil {
		return err
	}

	if err := uc.writeConfigFile(opts.OutputPath, deps); err != nil {
		return err
	}

	uc.showSuccessMessage(opts.OutputPath)

	return nil
}
