package cmd

import (
	"context"

	"raioz/internal/app"
	"raioz/internal/errors"

	"github.com/spf13/cobra"
)

var (
	initOutputPath string
)

var initCmd = &cobra.Command{
	Use:          "init",
	Short:        "Initialize a new .raioz.json configuration file",
	SilenceUsage: true, // Don't show usage/help on execution errors
	Long:         "Initialize a new .raioz.json configuration file interactively.",
	RunE: func(cmd *cobra.Command, args []string) (err error) {
		// Recover from panics in critical operation
		defer func() {
			if panicErr := errors.RecoverPanic("raioz init"); panicErr != nil {
				err = panicErr
			}
		}()

		// Create context for the operation
		ctx := cmd.Context()
		if ctx == nil {
			ctx = context.Background()
		}

		// Initialize dependencies and use case
		deps := app.NewDependencies()
		initUseCase := app.NewInitUseCase(deps)

		// Execute use case
		return initUseCase.Execute(ctx, app.InitOptions{
			OutputPath: initOutputPath,
		})
	},
}

func init() {
	initCmd.Flags().StringVarP(
		&initOutputPath,
		"output",
		"o",
		".raioz.json",
		"Output path for generated .raioz.json",
	)
}
