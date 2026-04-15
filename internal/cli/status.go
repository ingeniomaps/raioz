package cli

import (
	"context"

	"raioz/internal/app"
	"raioz/internal/errors"

	"github.com/spf13/cobra"
)

var (
	statusJSON bool
)

var statusCmd = &cobra.Command{
	Use:          "status",
	Short:        "Show project status",
	SilenceUsage: true, // Don't show usage/help on execution errors
	Long:         "Show detailed status information for all services.",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Recover from panics in critical operation. RecoverPanic logs the
		// panic internally; we discard the returned error because cobra
		// would re-print it after the deferred run completes.
		defer func() { _ = errors.RecoverPanic("raioz status") }()

		// Create context for the operation
		ctx := cmd.Context()
		if ctx == nil {
			ctx = context.Background()
		}

		// Initialize dependencies and use case
		deps := app.NewDependencies()
		statusUseCase := app.NewStatusUseCase(deps)

		// Execute use case
		return statusUseCase.Execute(ctx, app.StatusOptions{
			ProjectName: projectName,
			ConfigPath:  configPath,
			JSON:        statusJSON,
		})
	},
}

func init() {
	statusCmd.Flags().StringVarP(&configPath, "file", "f", "", "Path to config file")
	statusCmd.Flags().StringVarP(&projectName, "project", "p", "", "Project name (alternative to --file)")
	statusCmd.Flags().BoolVar(&statusJSON, "json", false, "Output status in JSON format")
}
