package cli

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
	Long:  "Compare your local raioz.yaml with a production Docker Compose file.",
	RunE: func(cmd *cobra.Command, args []string) error {
		configPath := ResolveConfigPath(configPath)
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
		"",
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
