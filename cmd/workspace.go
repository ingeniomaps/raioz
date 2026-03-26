package cmd

import (
	"raioz/internal/app"

	"github.com/spf13/cobra"
)

// workspaceCmd is the parent command for workspace operations
var workspaceCmd = &cobra.Command{
	Use:   "workspace",
	Short: "Manage workspaces",
	Long: `Manage workspaces for organizing multiple projects.

A workspace allows you to organize multiple projects and switch between them.
The active workspace is stored in ~/.raioz/active-workspace and can be used
as a default when working with projects.`,
}

// workspaceUseCmd is the command to set the active workspace
var workspaceUseCmd = &cobra.Command{
	Use:   "use <workspace-name>",
	Short: "Set the active workspace",
	Long: `Set the active workspace to use for future commands.

If the workspace does not exist, it will be created automatically.
The active workspace will be loaded automatically when using raioz commands
if no project is explicitly specified.

Example:
  raioz workspace use empresa-x
  raioz workspace use billing-platform`,
	Args: cobra.ExactArgs(1),
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
	Long: `List all available workspaces and show which one is currently active.

Example:
  raioz workspace list`,
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
