package cmd

import (
	"context"

	"raioz/internal/app"
	"raioz/internal/errors"

	"github.com/spf13/cobra"
)

var checkCmd = &cobra.Command{
	Use:          "check",
	Short:        "Check for alignment issues between config and state",
	SilenceUsage: true, // Don't show usage/help on execution errors
	Long:         "Check if the current configuration aligns with the saved state.",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Recover from panics in critical operation
		defer func() {
			if panicErr := errors.RecoverPanic("raioz check"); panicErr != nil {
				// Error is returned via named return value
			}
		}()

		// Create context for the operation
		ctx := cmd.Context()
		if ctx == nil {
			ctx = context.Background()
		}

		// Initialize dependencies and use case
		deps := app.NewDependencies()
		checkUseCase := app.NewCheckUseCase(deps)

		// Execute use case
		return checkUseCase.Execute(ctx, app.CheckOptions{
			ProjectName: projectName,
			ConfigPath:  configPath,
		})
	},
}

func init() {
	checkCmd.Flags().StringVarP(&configPath, "file", "f", ".raioz.json", "Path to config file")
	checkCmd.Flags().StringVarP(&projectName, "project", "p", "", "Project name (alternative to --file)")
}
