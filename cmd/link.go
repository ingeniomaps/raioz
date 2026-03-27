package cmd

import (
	"raioz/internal/app"

	"github.com/spf13/cobra"
)

// linkCmd is the parent command for link operations
var linkCmd = &cobra.Command{
	Use:   "link",
	Short: "Manage service symlinks for external editing",
	Long:  "Manage symlinks from Raioz workspace to external paths.",
}

// linkAddCmd is the command to create a symlink
var linkAddCmd = &cobra.Command{
	Use:   "add <service> <external-path>",
	Short: "Create a symlink from workspace to external path",
	Long:  "Create a symlink from the Raioz workspace service directory to an external path.",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		deps := app.NewDependencies()
		useCase := app.NewLinkUseCase(deps)
		return useCase.Add(args[0], args[1], configPath)
	},
}

// linkRemoveCmd is the command to remove a symlink
var linkRemoveCmd = &cobra.Command{
	Use:     "remove <service>",
	Aliases: []string{"rm", "unlink"},
	Short:   "Remove a service symlink",
	Long:    "Remove a symlink from a service.",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		deps := app.NewDependencies()
		useCase := app.NewLinkUseCase(deps)
		return useCase.Remove(args[0], configPath)
	},
}

// linkListCmd is the command to list linked services
var linkListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all linked services",
	Long:  "List all services that are currently linked to external paths.",
	RunE: func(cmd *cobra.Command, args []string) error {
		deps := app.NewDependencies()
		useCase := app.NewLinkUseCase(deps)
		return useCase.List(configPath)
	},
}

func init() {
	// Add subcommands to link command
	linkCmd.AddCommand(linkAddCmd)
	linkCmd.AddCommand(linkRemoveCmd)
	linkCmd.AddCommand(linkListCmd)

	// Note: linkCmd is added to rootCmd in root.go init() to avoid circular dependencies
}
