package cmd

import (
	"raioz/internal/app"

	"github.com/spf13/cobra"
)

// workspaceCmd is the parent command for workspace operations
var workspaceCmd = &cobra.Command{
	Use:   "workspace",
	Short: "Manage workspaces",
	Long:  "Manage workspaces for organizing multiple projects.",
}

// workspaceUseCmd is the command to set the active workspace
var workspaceUseCmd = &cobra.Command{
	Use:   "use <workspace-name>",
	Short: "Set the active workspace",
	Long:  "Set the active workspace to use for future commands.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		deps := app.NewDependencies()
		useCase := app.NewWorkspaceUseCase(deps)
		return useCase.Use(args[0])
	},
}

// workspaceListCmd is the command to list available workspaces
var workspaceListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available workspaces",
	Long:  "List all available workspaces and show which one is currently active.",
	RunE: func(cmd *cobra.Command, args []string) error {
		deps := app.NewDependencies()
		useCase := app.NewWorkspaceUseCase(deps)
		return useCase.List()
	},
}

func init() {
	// Add subcommands to workspace command
	workspaceCmd.AddCommand(workspaceUseCmd)
	workspaceCmd.AddCommand(workspaceListCmd)

	// Note: workspaceCmd is added to rootCmd in root.go init() to avoid circular dependencies
}
