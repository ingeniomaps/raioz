package cli

import (
	"raioz/internal/app"

	"github.com/spf13/cobra"
)

// ignoreCmd is the parent command for ignore operations
var ignoreCmd = &cobra.Command{
	Use:   "ignore",
	Short: "Manage ignored services",
	Long:  "Manage services that should be ignored during dependency resolution.",
}

// ignoreAddCmd is the command to add a service to the ignore list
var ignoreAddCmd = &cobra.Command{
	Use:   "add <service> [service...]",
	Short: "Add a service to the ignore list",
	Long: "Add one or more services to the ignore list. " +
		"The services will not be cloned, built, or started during raioz up.",
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		deps := app.NewDependencies()
		useCase := app.NewIgnoreUseCase(deps)
		for _, svc := range args {
			if err := useCase.Add(svc, configPath); err != nil {
				return err
			}
		}
		return nil
	},
}

// ignoreRemoveCmd is the command to remove a service from the ignore list
var ignoreRemoveCmd = &cobra.Command{
	Use:     "remove <service> [service...]",
	Aliases: []string{"rm"},
	Short:   "Remove a service from the ignore list",
	Long: "Remove one or more services from the ignore list. " +
		"The services will be processed normally on the next raioz up.",
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		deps := app.NewDependencies()
		useCase := app.NewIgnoreUseCase(deps)
		for _, svc := range args {
			if err := useCase.Remove(svc); err != nil {
				return err
			}
		}
		return nil
	},
}

// ignoreListCmd is the command to list ignored services
var ignoreListCmd = &cobra.Command{
	Use:   "list",
	Short: "List ignored services",
	Long:  "List all services that are currently in the ignore list.",
	RunE: func(cmd *cobra.Command, args []string) error {
		deps := app.NewDependencies()
		useCase := app.NewIgnoreUseCase(deps)
		return useCase.List()
	},
}

func init() {
	// Add subcommands to ignore command
	ignoreCmd.AddCommand(ignoreAddCmd)
	ignoreCmd.AddCommand(ignoreRemoveCmd)
	ignoreCmd.AddCommand(ignoreListCmd)

	// Note: ignoreCmd is added to rootCmd in root.go init() to avoid circular dependencies
}
