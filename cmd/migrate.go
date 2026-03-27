package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"raioz/internal/errors"
	"raioz/internal/output"
	"raioz/internal/production"

	"github.com/spf13/cobra"
)

var (
	migrateComposePath string
	migrateOutputPath  string
	migrateProjectName string
	migrateNetworkName string
)

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Convert production configuration to .raioz.json",
	Long:  "Convert a production Docker Compose file to .raioz.json format.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if migrateComposePath == "" {
			return errors.New(
				errors.ErrCodeInvalidConfig,
				"Compose path required",
			).WithSuggestion(
				"Please specify the path to the docker-compose.yml file using --compose flag",
			)
		}

		if migrateProjectName == "" {
			return errors.New(
				errors.ErrCodeInvalidConfig,
				"Project name required",
			).WithSuggestion(
				"Please specify the project name using --project flag",
			)
		}

		// Load production configuration
		prodConfig, err := production.LoadComposeFile(migrateComposePath)
		if err != nil {
			return errors.New(
				errors.ErrCodeInvalidConfig,
				fmt.Sprintf("Failed to load compose file from %s", migrateComposePath),
			).WithError(err).WithContext("compose_path", migrateComposePath)
		}

		// Migrate to .raioz.json format
		deps, err := production.MigrateComposeToDeps(
			prodConfig,
			migrateProjectName,
			migrateNetworkName,
		)
		if err != nil {
			return errors.New(
				errors.ErrCodeInvalidConfig,
				"Failed to migrate configuration",
			).WithError(err)
		}

		// Enhance with suggestions
		production.EnhanceMigratedDeps(deps)

		// Validate migrated configuration
		warnings := production.ValidateMigratedDeps(deps)
		for _, warning := range warnings {
			output.PrintWarning(warning)
		}

		// Write output
		outputData, err := json.MarshalIndent(deps, "", "  ")
		if err != nil {
			return errors.New(
				errors.ErrCodeInvalidConfig,
				"Failed to marshal .raioz.json",
			).WithError(err)
		}

		if migrateOutputPath == "" {
			migrateOutputPath = ".raioz.json"
		}

		if err := os.WriteFile(migrateOutputPath, outputData, 0644); err != nil {
			return errors.New(
				errors.ErrCodeInvalidConfig,
				fmt.Sprintf("Failed to write .raioz.json to %s", migrateOutputPath),
			).WithError(err).WithContext("output_path", migrateOutputPath)
		}

		output.PrintSuccess(fmt.Sprintf("Migrated configuration written to %s", migrateOutputPath))

		if len(warnings) > 0 {
			output.PrintWarning(fmt.Sprintf("Generated with %d warnings. Please review and adjust.", len(warnings)))
		} else {
			output.PrintSuccess("Migration completed successfully!")
		}

		return nil
	},
}

func init() {
	migrateCmd.Flags().StringVarP(
		&migrateComposePath,
		"compose",
		"c",
		"",
		"Path to docker-compose.yml file (required)",
	)
	migrateCmd.Flags().StringVarP(
		&migrateOutputPath,
		"output",
		"o",
		".raioz.json",
		"Output path for generated .raioz.json",
	)
	migrateCmd.Flags().StringVarP(
		&migrateProjectName,
		"project",
		"p",
		"",
		"Project name (required)",
	)
	migrateCmd.Flags().StringVar(
		&migrateNetworkName,
		"network",
		"",
		"Network name (defaults to {project-name}-network)",
	)
	migrateCmd.MarkFlagRequired("compose")
	migrateCmd.MarkFlagRequired("project")
	// Note: migrateCmd is added to rootCmd in root.go init() to avoid circular dependencies
}
