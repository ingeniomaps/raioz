package cli

import (
	"raioz/internal/app"

	"github.com/spf13/cobra"
)

var workspaceCmd = &cobra.Command{
	Use:   "workspace",
	Short: "Manage workspaces",
	Long:  "Manage workspaces for organizing multiple projects.",
	RunE: func(cmd *cobra.Command, args []string) error {
		deps := app.NewDependencies()
		useCase := app.NewWorkspaceUseCase(deps)
		return useCase.Current()
	},
}

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

var workspaceDeleteCmd = &cobra.Command{
	Use:   "delete <workspace-name>",
	Short: "Delete a workspace",
	Long:  "Delete a workspace and all its contents.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		deps := app.NewDependencies()
		useCase := app.NewWorkspaceUseCase(deps)
		return useCase.Delete(args[0])
	},
}

func init() {
	workspaceCmd.AddCommand(workspaceUseCmd)
	workspaceCmd.AddCommand(workspaceListCmd)
	workspaceCmd.AddCommand(workspaceDeleteCmd)
}
