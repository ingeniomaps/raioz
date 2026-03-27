package cmd

import (
	"raioz/internal/app"

	"github.com/spf13/cobra"
)

var overridePath string

var overrideCmd = &cobra.Command{
	Use:   "override <service>",
	Short: "Override a service with a local path",
	Long:  "Override a service to use a local path instead of the Git repository or image defined in .raioz.json.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		deps := app.NewDependencies()
		useCase := app.NewOverrideUseCase(deps)
		return useCase.Apply(args[0], overridePath, configPath)
	},
}

var overrideListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all service overrides",
	Long:  "List all registered service overrides from ~/.raioz/overrides.json.",
	RunE: func(cmd *cobra.Command, args []string) error {
		deps := app.NewDependencies()
		useCase := app.NewOverrideUseCase(deps)
		return useCase.List()
	},
}

var overrideRemoveCmd = &cobra.Command{
	Use:     "remove <service>",
	Aliases: []string{"rm"},
	Short:   "Remove a service override",
	Long:    "Remove a service override. The service will fall back to the configuration in .raioz.json.",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		deps := app.NewDependencies()
		useCase := app.NewOverrideUseCase(deps)
		return useCase.Remove(args[0])
	},
}

func init() {
	overrideCmd.Flags().StringVar(&overridePath, "path", "", "Local path to override the service (required)")
	overrideCmd.AddCommand(overrideListCmd)
	overrideCmd.AddCommand(overrideRemoveCmd)
}
