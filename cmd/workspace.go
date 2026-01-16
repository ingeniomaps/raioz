package cmd

import (
	"fmt"

	"raioz/internal/audit"
	"raioz/internal/output"
	"raioz/internal/root"
	"raioz/internal/workspace"

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
		workspaceName := args[0]

		// Validate workspace name (basic validation)
		if workspaceName == "" {
			return fmt.Errorf("workspace name cannot be empty")
		}

		// Check if workspace exists
		exists, err := workspace.WorkspaceExists(workspaceName)
		if err != nil {
			return fmt.Errorf("failed to check workspace existence: %w", err)
		}

		if !exists {
			// Create workspace by resolving it (this creates the directory structure)
			ws, err := workspace.Resolve(workspaceName)
			if err != nil {
				return fmt.Errorf("failed to create workspace: %w", err)
			}
			output.PrintInfo(fmt.Sprintf("Created workspace: %s", workspaceName))
			_ = ws // Use workspace to ensure it's created
		} else {
			output.PrintInfo(fmt.Sprintf("Workspace %s already exists", workspaceName))
		}

		// Load raioz.root.json if it exists (to validate it)
		ws, err := workspace.Resolve(workspaceName)
		if err != nil {
			return fmt.Errorf("failed to resolve workspace: %w", err)
		}

		if root.Exists(ws) {
			rootConfig, err := root.Load(ws)
			if err != nil {
				return fmt.Errorf("failed to load raioz.root.json: %w", err)
			}
			if rootConfig != nil {
				output.PrintInfo(fmt.Sprintf("Loaded raioz.root.json for workspace %s", workspaceName))
			}
		}

		// Get old workspace before setting new one
		oldWorkspace, _ := workspace.GetActiveWorkspace()

		// Set as active workspace
		if err := workspace.SetActiveWorkspace(workspaceName); err != nil {
			return fmt.Errorf("failed to set active workspace: %w", err)
		}

		// Log audit event if workspace changed
		if oldWorkspace != workspaceName {
			if err := audit.LogWorkspaceChanged(oldWorkspace, workspaceName); err != nil {
				// Log audit error but don't fail the command
				output.PrintWarning(fmt.Sprintf("Failed to log audit event: %v", err))
			}
		}

		output.PrintSuccess(fmt.Sprintf("Active workspace set to: %s", workspaceName))
		return nil
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
		// Get active workspace
		activeWorkspace, err := workspace.GetActiveWorkspace()
		if err != nil {
			return fmt.Errorf("failed to get active workspace: %w", err)
		}

		// List all workspaces
		workspaces, err := workspace.ListWorkspaces()
		if err != nil {
			return fmt.Errorf("failed to list workspaces: %w", err)
		}

		if len(workspaces) == 0 {
			fmt.Println("No workspaces found.")
			fmt.Println("Create a workspace by running: raioz workspace use <name>")
			return nil
		}

		// Print workspaces
		fmt.Println("Available workspaces:")
		for _, ws := range workspaces {
			marker := " "
			if ws == activeWorkspace {
				marker = "*"
			}
			fmt.Printf("  %s %s", marker, ws)
			if ws == activeWorkspace {
				fmt.Print(" (active)")
			}
			fmt.Println()
		}

		if activeWorkspace == "" {
			fmt.Println("\nNo active workspace set.")
			fmt.Println("Set an active workspace by running: raioz workspace use <name>")
		}

		return nil
	},
}

func init() {
	// Add subcommands to workspace command
	workspaceCmd.AddCommand(workspaceUseCmd)
	workspaceCmd.AddCommand(workspaceListCmd)

	// Note: workspaceCmd is added to rootCmd in root.go init() to avoid circular dependencies
}
