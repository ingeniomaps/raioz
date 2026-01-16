package cmd

import (
	"fmt"

	"raioz/internal/config"
	"raioz/internal/ignore"
	"raioz/internal/output"

	"github.com/spf13/cobra"
)

// ignoreCmd is the parent command for ignore operations
var ignoreCmd = &cobra.Command{
	Use:   "ignore",
	Short: "Manage ignored services",
	Long: `Manage services that should be ignored during dependency resolution.

Ignored services will not be cloned, built, or started. However, if other
services depend on ignored services, a warning will be shown.

Example:
  raioz ignore add service-name
  raioz ignore remove service-name
  raioz ignore list`,
}

// ignoreAddCmd is the command to add a service to the ignore list
var ignoreAddCmd = &cobra.Command{
	Use:   "add <service>",
	Short: "Add a service to the ignore list",
	Long: `Add a service to the ignore list. The service will not be cloned,
built, or started during raioz up.

Example:
  raioz ignore add old-service`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		serviceName := args[0]

		// Validate service name (basic validation)
		if serviceName == "" {
			return fmt.Errorf("service name cannot be empty")
		}

		// Check if service is already ignored
		isIgnored, err := ignore.IsIgnored(serviceName)
		if err != nil {
			return fmt.Errorf("failed to check if service is ignored: %w", err)
		}

		if isIgnored {
			output.PrintInfo(fmt.Sprintf("Service '%s' is already ignored", serviceName))
			return nil
		}

		// Add to ignore list
		if err := ignore.AddService(serviceName); err != nil {
			return fmt.Errorf("failed to add service to ignore list: %w", err)
		}

		output.PrintSuccess(fmt.Sprintf("Service '%s' added to ignore list", serviceName))
		output.PrintInfo("The service will not be started on the next 'raioz up'")

		// Check if service exists in current config and warn about dependencies
		deps, _, _ := config.LoadDeps(configPath)
		if deps != nil {
			if _, exists := deps.Services[serviceName]; exists {
				// Check if other services depend on this one
				var dependents []string
				for name, svc := range deps.Services {
					for _, dep := range svc.Docker.DependsOn {
						if dep == serviceName {
							dependents = append(dependents, name)
							break
						}
					}
				}
				if len(dependents) > 0 {
					output.PrintWarning(
						fmt.Sprintf(
							"Service '%s' is required by: %v. These services may fail without it.",
							serviceName,
							dependents,
						),
					)
				}
			}
		}

		return nil
	},
}

// ignoreRemoveCmd is the command to remove a service from the ignore list
var ignoreRemoveCmd = &cobra.Command{
	Use:     "remove <service>",
	Aliases: []string{"rm"},
	Short:   "Remove a service from the ignore list",
	Long: `Remove a service from the ignore list. The service will be processed
normally on the next raioz up.

Example:
  raioz ignore remove service-name`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		serviceName := args[0]

		// Check if service is ignored
		isIgnored, err := ignore.IsIgnored(serviceName)
		if err != nil {
			return fmt.Errorf("failed to check if service is ignored: %w", err)
		}

		if !isIgnored {
			output.PrintInfo(fmt.Sprintf("Service '%s' is not in the ignore list", serviceName))
			return nil
		}

		// Remove from ignore list
		if err := ignore.RemoveService(serviceName); err != nil {
			return fmt.Errorf("failed to remove service from ignore list: %w", err)
		}

		output.PrintSuccess(fmt.Sprintf("Service '%s' removed from ignore list", serviceName))
		output.PrintInfo("The service will be processed normally on the next 'raioz up'")

		return nil
	},
}

// ignoreListCmd is the command to list ignored services
var ignoreListCmd = &cobra.Command{
	Use:   "list",
	Short: "List ignored services",
	Long: `List all services that are currently in the ignore list.

Example:
  raioz ignore list`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ignoredServices, err := ignore.GetIgnoredServices()
		if err != nil {
			return fmt.Errorf("failed to get ignored services: %w", err)
		}

		if len(ignoredServices) == 0 {
			fmt.Println("No services in ignore list.")
			return nil
		}

		fmt.Println("Ignored services:")
		for _, name := range ignoredServices {
			fmt.Printf("  - %s\n", name)
		}

		return nil
	},
}

func init() {
	// Add subcommands to ignore command
	ignoreCmd.AddCommand(ignoreAddCmd)
	ignoreCmd.AddCommand(ignoreRemoveCmd)
	ignoreCmd.AddCommand(ignoreListCmd)

	// Note: ignoreCmd is added to rootCmd in root.go init() to avoid circular dependencies
}
