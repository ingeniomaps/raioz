package cmd

import (
	"fmt"
	"path/filepath"

	"raioz/internal/audit"
	"raioz/internal/config"
	"raioz/internal/output"
	"raioz/internal/override"

	"github.com/spf13/cobra"
)

var overridePath string

var overrideCmd = &cobra.Command{
	Use:   "override <service>",
	Short: "Override a service with a local path",
	Long: `Override a service to use a local path instead of the Git repository or image
defined in .raioz.json.

This command does NOT modify .raioz.json. Instead, it registers an override
in ~/.raioz/overrides.json that takes precedence over .raioz.json.

The override will be automatically reverted if the path no longer exists.

Example:
  raioz override orders --path ~/dev/orders
  raioz override api --path /opt/custom/api`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		serviceName := args[0]

		if overridePath == "" {
			return fmt.Errorf("--path is required")
		}

		// Validate path
		absPath, err := filepath.Abs(overridePath)
		if err != nil {
			return fmt.Errorf("failed to resolve override path: %w", err)
		}

		if err := override.ValidateOverridePath(absPath); err != nil {
			return fmt.Errorf("invalid override path: %w", err)
		}

		// Check if service exists in current config (optional check)
		var projectName string
		if configPath != "" {
			deps, _, _ := config.LoadDeps(configPath)
			if deps != nil {
				projectName = deps.Project.Name
				if _, ok := deps.Services[serviceName]; !ok {
					output.PrintWarning(
						fmt.Sprintf(
							"Service '%s' not found in current .raioz.json. "+
								"Override will still be registered.",
							serviceName,
						),
					)
				}
			}
		}

		// Check if override already exists
		hasOverride, err := override.HasOverride(serviceName)
		if err != nil {
			return fmt.Errorf("failed to check existing override: %w", err)
		}

		if hasOverride {
			existingOverride, err := override.GetOverride(serviceName)
			if err != nil {
				return fmt.Errorf("failed to get existing override: %w", err)
			}
			output.PrintWarning(
				fmt.Sprintf(
					"Service '%s' already has an override pointing to: %s",
					serviceName,
					existingOverride.Path,
				),
			)
			output.PrintInfo("Overriding existing override...")
		}

		// Add override
		overrideConfig := override.Override{
			Path:   absPath,
			Mode:   "local",
			Source: "external",
		}

		if err := override.AddOverride(serviceName, overrideConfig); err != nil {
			return fmt.Errorf("failed to add override: %w", err)
		}

		// Log audit event
		if err := audit.LogOverrideApplied(serviceName, absPath); err != nil {
			// Log audit error but don't fail the command
			output.PrintWarning(fmt.Sprintf("Failed to log audit event: %v", err))
		}

		output.PrintSuccess(
			fmt.Sprintf(
				"Override registered for service '%s' -> %s",
				serviceName,
				absPath,
			),
		)

		if projectName != "" {
			output.PrintInfo(
				fmt.Sprintf(
					"Run 'raioz up' in project '%s' to apply the override.",
					projectName,
				),
			)
		} else {
			output.PrintInfo("Run 'raioz up' to apply the override.")
		}

		return nil
	},
}

var overrideListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all service overrides",
	Long: `List all registered service overrides from ~/.raioz/overrides.json.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		overrides, err := override.LoadOverrides()
		if err != nil {
			return fmt.Errorf("failed to load overrides: %w", err)
		}

		if len(overrides) == 0 {
			fmt.Println("No overrides registered.")
			return nil
		}

		fmt.Println("Registered overrides:")
		fmt.Println()
		for serviceName, overrideConfig := range overrides {
			// Validate path exists
			pathStatus := "✓"
			if err := override.ValidateOverridePath(overrideConfig.Path); err != nil {
				pathStatus = "✗ (path does not exist)"
			}

			fmt.Printf("  %s:\n", serviceName)
			fmt.Printf("    Path:   %s %s\n", overrideConfig.Path, pathStatus)
			fmt.Printf("    Mode:   %s\n", overrideConfig.Mode)
			fmt.Printf("    Source: %s\n", overrideConfig.Source)
			fmt.Println()
		}

		// Clean invalid overrides
		removed, err := override.CleanInvalidOverrides()
		if err != nil {
			output.PrintWarning(fmt.Sprintf("Failed to clean invalid overrides: %v", err))
		} else if len(removed) > 0 {
			fmt.Printf("⚠️  Removed %d invalid override(s): %v\n", len(removed), removed)
		}

		return nil
	},
}

var overrideRemoveCmd = &cobra.Command{
	Use:     "remove <service>",
	Aliases: []string{"rm"},
	Short:   "Remove a service override",
	Long: `Remove a service override. The service will fall back to the configuration
in .raioz.json.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		serviceName := args[0]

		hasOverride, err := override.HasOverride(serviceName)
		if err != nil {
			return fmt.Errorf("failed to check override: %w", err)
		}

		if !hasOverride {
			return fmt.Errorf("no override found for service '%s'", serviceName)
		}

		if err := override.RemoveOverride(serviceName); err != nil {
			return fmt.Errorf("failed to remove override: %w", err)
		}

		// Log audit event
		if err := audit.LogOverrideReverted(serviceName, "user removed override"); err != nil {
			// Log audit error but don't fail the command
			output.PrintWarning(fmt.Sprintf("Failed to log audit event: %v", err))
		}

		output.PrintSuccess(fmt.Sprintf("Override removed for service '%s'", serviceName))
		output.PrintInfo("Run 'raioz up' to apply the change.")

		return nil
	},
}

func init() {
	overrideCmd.Flags().StringVar(&overridePath, "path", "", "Local path to override the service (required)")
	overrideCmd.AddCommand(overrideListCmd)
	overrideCmd.AddCommand(overrideRemoveCmd)
}
