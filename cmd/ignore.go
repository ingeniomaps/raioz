package cmd

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
	Use:   "add <service>",
	Short: "Add a service to the ignore list",
	Long:  "Add a service to the ignore list. The service will not be cloned, built, or started during raioz up.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		deps := app.NewDependencies()
		useCase := app.NewIgnoreUseCase(deps)
		return useCase.Add(args[0], configPath)
	},
}

// ignoreRemoveCmd is the command to remove a service from the ignore list
var ignoreRemoveCmd = &cobra.Command{
	Use:     "remove <service>",
	Aliases: []string{"rm"},
	Short:   "Remove a service from the ignore list",
	Long:    "Remove a service from the ignore list. The service will be processed normally on the next raioz up.",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		deps := app.NewDependencies()
		useCase := app.NewIgnoreUseCase(deps)
		return useCase.Remove(args[0])
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
