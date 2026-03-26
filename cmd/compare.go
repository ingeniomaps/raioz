package cmd

import (
	"raioz/internal/app"

	"github.com/spf13/cobra"
)

var (
	compareProductionPath string
	compareJSONOutput     bool
)

var compareCmd = &cobra.Command{
	Use:   "compare",
	Short: "Compare local configuration with production",
	Long: `Compare your local .raioz.json with a production Docker Compose file
to identify differences in images, ports, volumes, and dependencies.

This helps ensure that your local development environment matches production
as closely as possible.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		deps := app.NewDependencies()
		useCase := app.NewCompareUseCase(deps)
		return useCase.Execute(app.CompareOptions{
			ConfigPath:     configPath,
			ProductionPath: compareProductionPath,
			JSONOutput:     compareJSONOutput,
		})
	},
}

func init() {
	compareCmd.Flags().StringVarP(
		&configPath,
		"file",
		"f",
		".raioz.json",
		"Path to local config file",
	)
	compareCmd.Flags().StringVarP(
		&compareProductionPath,
		"production",
		"p",
		"",
		"Path to production docker-compose.yml (required)",
	)
	compareCmd.Flags().BoolVar(
		&compareJSONOutput,
		"json",
		false,
		"Output results in JSON format",
	)
	compareCmd.MarkFlagRequired("production")
	// Note: compareCmd is added to rootCmd in root.go init() to avoid circular dependencies
}
