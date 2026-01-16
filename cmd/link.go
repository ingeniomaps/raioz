package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"raioz/internal/config"
	"raioz/internal/link"
	"raioz/internal/output"
	"raioz/internal/workspace"

	"github.com/spf13/cobra"
)

// linkCmd is the parent command for link operations
var linkCmd = &cobra.Command{
	Use:   "link",
	Short: "Manage service symlinks for external editing",
	Long: `Manage symlinks from Raioz workspace to external paths.

This allows you to edit service code in an external location while Raioz
manages the service from its workspace directory.

Example:
  raioz link payments ~/dev/payments
  raioz unlink payments`,
}

// linkAddCmd is the command to create a symlink
var linkAddCmd = &cobra.Command{
	Use:   "add <service> <external-path>",
	Short: "Create a symlink from workspace to external path",
	Long: `Create a symlink from the Raioz workspace service directory to an
external path. This allows you to edit the service code in the external
location while Raioz uses it from the workspace.

The external path must exist and be a directory. If the service path already
exists as a directory (not a symlink), the command will fail.

Example:
  raioz link add payments ~/dev/payments`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		serviceName := args[0]
		externalPath := args[1]

		// Resolve external path to absolute
		absExternalPath, err := filepath.Abs(externalPath)
		if err != nil {
			return fmt.Errorf("failed to resolve external path: %w", err)
		}

		// Load config to get project name
		deps, _, err := config.LoadDeps(configPath)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		// Resolve workspace
		ws, err := workspace.Resolve(deps.Project.Name)
		if err != nil {
			return fmt.Errorf("failed to resolve workspace: %w", err)
		}

		// Check if service exists in config
		svc, exists := deps.Services[serviceName]
		if !exists {
			return fmt.Errorf("service '%s' not found in configuration", serviceName)
		}

		// Get service path in workspace
		servicePath := workspace.GetServicePath(ws, serviceName, svc)

		// Check if service path already exists as a directory (not symlink)
		if info, err := os.Stat(servicePath); err == nil {
			if info.IsDir() {
				// Check if it's a symlink
				isLinked, target, err := link.IsLinked(servicePath)
				if err != nil {
					return fmt.Errorf("failed to check if service is linked: %w", err)
				}
				if !isLinked {
					return fmt.Errorf(
						"service path already exists as a directory: %s\n"+
							"To create a symlink, you must first remove or move the existing directory",
						servicePath,
					)
				}
				// Already linked, check if it points to the same target
				absTarget, err := filepath.Abs(target)
				if err != nil {
					return fmt.Errorf("failed to resolve existing target: %w", err)
				}
				if absTarget == absExternalPath {
					output.PrintInfo(fmt.Sprintf("Service '%s' is already linked to: %s", serviceName, absExternalPath))
					return nil
				}
				return fmt.Errorf(
					"service '%s' is already linked to: %s\n"+
						"Use 'raioz link remove %s' first to unlink it",
					serviceName, target, serviceName,
				)
			}
		}

		// Create symlink
		if err := link.CreateLink(servicePath, absExternalPath); err != nil {
			return fmt.Errorf("failed to create symlink: %w", err)
		}

		output.PrintSuccess(fmt.Sprintf("Linked service '%s' to: %s", serviceName, absExternalPath))
		output.PrintInfo(fmt.Sprintf("Service path: %s", servicePath))

		return nil
	},
}

// linkRemoveCmd is the command to remove a symlink
var linkRemoveCmd = &cobra.Command{
	Use:     "remove <service>",
	Aliases: []string{"rm", "unlink"},
	Short:   "Remove a service symlink",
	Long: `Remove a symlink from a service. This will remove the symlink but
will not delete the service from the workspace or the external directory.

After removing the symlink, the service will need to be cloned again if it
was originally a Git repository.

Example:
  raioz link remove payments
  raioz link unlink payments`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		serviceName := args[0]

		// Load config to get project name
		deps, _, err := config.LoadDeps(configPath)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		// Resolve workspace
		ws, err := workspace.Resolve(deps.Project.Name)
		if err != nil {
			return fmt.Errorf("failed to resolve workspace: %w", err)
		}

		// Check if service exists in config
		svc, exists := deps.Services[serviceName]
		if !exists {
			return fmt.Errorf("service '%s' not found in configuration", serviceName)
		}

		// Get service path in workspace
		servicePath := workspace.GetServicePath(ws, serviceName, svc)

		// Check if service is linked
		isLinked, target, err := link.IsLinked(servicePath)
		if err != nil {
			return fmt.Errorf("failed to check if service is linked: %w", err)
		}

		if !isLinked {
			output.PrintInfo(fmt.Sprintf("Service '%s' is not linked", serviceName))
			return nil
		}

		// Remove symlink
		if err := link.RemoveLink(servicePath); err != nil {
			return fmt.Errorf("failed to remove symlink: %w", err)
		}

		output.PrintSuccess(fmt.Sprintf("Removed symlink for service '%s' (was pointing to: %s)", serviceName, target))
		output.PrintInfo("The external directory was not deleted")

		return nil
	},
}

// linkListCmd is the command to list linked services
var linkListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all linked services",
	Long: `List all services that are currently linked to external paths.

Example:
  raioz link list`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Load config to get project name
		deps, _, err := config.LoadDeps(configPath)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		// Resolve workspace
		ws, err := workspace.Resolve(deps.Project.Name)
		if err != nil {
			return fmt.Errorf("failed to resolve workspace: %w", err)
		}

		var linkedServices []struct {
			name   string
			target string
		}

		// Check each service
		for name, svc := range deps.Services {
			servicePath := workspace.GetServicePath(ws, name, svc)
			isLinked, target, err := link.IsLinked(servicePath)
			if err != nil {
				// Skip on error (service might not exist yet)
				continue
			}
			if isLinked {
				linkedServices = append(linkedServices, struct {
					name   string
					target string
				}{name, target})
			}
		}

		if len(linkedServices) == 0 {
			fmt.Println("No services are currently linked.")
			return nil
		}

		fmt.Println("Linked services:")
		for _, linked := range linkedServices {
			fmt.Printf("  %s -> %s\n", linked.name, linked.target)
		}

		return nil
	},
}

func init() {
	// Add subcommands to link command
	linkCmd.AddCommand(linkAddCmd)
	linkCmd.AddCommand(linkRemoveCmd)
	linkCmd.AddCommand(linkListCmd)

	// Note: linkCmd is added to rootCmd in root.go init() to avoid circular dependencies
}
