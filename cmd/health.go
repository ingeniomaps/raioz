package cmd

import (
	"context"

	"raioz/internal/app"
	"raioz/internal/errors"

	"github.com/spf13/cobra"
)

var healthCmd = &cobra.Command{
	Use:          "health",
	Short:        "Check project health and manage local project lifecycle",
	SilenceUsage: true, // Don't show usage/help on execution errors
	Long: `Check if the local project is running and manage its lifecycle.

This command:
1. Checks if the project is running (using health command if defined)
2. If not running, starts it with the 'up' command
3. If running, does nothing
4. If needs to be stopped, stops it with the 'down' command

The health command can be defined in .raioz.json under project.commands.health`,
	RunE: func(cmd *cobra.Command, args []string) (err error) {
		// Recover from panics
		defer func() {
			if panicErr := errors.RecoverPanic("raioz health"); panicErr != nil {
				err = panicErr
			}
		}()

		// Create context
		ctx := cmd.Context()
		if ctx == nil {
			ctx = context.Background()
		}

		deps := app.NewDependencies()
		useCase := app.NewHealthUseCase(deps)
		return useCase.Execute(ctx, app.HealthOptions{
			ConfigPath: configPath,
		})
	},
}

func init() {
	healthCmd.Flags().StringVarP(&configPath, "file", "f", ".raioz.json", "Path to config file")
	rootCmd.AddCommand(healthCmd)
}
