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
	Use:   "status [service...]",
	Short: "Show project status",
	Long: "Show detailed status information for services. Pass service / " +
		"dependency names to filter the report; without args, everything is " +
		"shown.",
	Args:         cobra.ArbitraryArgs,
	SilenceUsage: true,
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

		// Meta-orchestrator dispatches to sub-projects.
		resolved := ResolveConfigPath(configPath)
		if handled, metaErr := tryHandleMeta(
			ctx, resolved, "status", nil, metaProfiles, app.MetaUpOptions{},
		); handled {
			return metaErr
		}

		// Initialize dependencies and use case
		deps := newDependencies()
		statusUseCase := app.NewStatusUseCase(deps)

		// Execute use case
		return statusUseCase.Execute(ctx, app.StatusOptions{
			ProjectName: projectName,
			ConfigPath:  configPath,
			JSON:        statusJSON,
			Services:    args,
		})
	},
}

func init() {
	statusCmd.Flags().StringVarP(&configPath, "file", "f", "", "Path to config file")
	statusCmd.Flags().StringVarP(&projectName, "project", "p", "", "Project name (alternative to --file)")
	statusCmd.Flags().BoolVar(&statusJSON, "json", false, "Output status in JSON format")
	statusCmd.Flags().StringSliceVar(&metaProfiles, "meta-profile", nil,
		"Report only meta sub-projects tagged with these profiles (kind: meta only). Repeatable.")
}
