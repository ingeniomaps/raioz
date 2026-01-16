package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"raioz/internal/config"
	"raioz/internal/errors"
	"raioz/internal/output"
	"raioz/internal/production"

	"github.com/spf13/cobra"
)

var (
	compareProductionPath string
	compareJSONOutput     bool
)

var compareCmd = &cobra.Command{
	Use:   "compare",
	Short: "Compare local configuration with production",
	Long: `Compare your local .raioz.json with a production Docker Compose file
to identify differences in images, ports, volumes, and dependencies.

This helps ensure that your local development environment matches production
as closely as possible.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if compareProductionPath == "" {
			return errors.New(
				errors.ErrCodeInvalidConfig,
				"Production path required",
			).WithSuggestion(
				"Please specify the path to the production docker-compose.yml file using --production flag",
			)
		}

		// Load local configuration
		deps, warnings, err := config.LoadDeps(configPath)
		if err != nil {
			return errors.New(
				errors.ErrCodeInvalidConfig,
				fmt.Sprintf("Failed to load local config from %s", configPath),
			).WithError(err).WithContext("config_path", configPath)
		}

		// Print warnings
		for _, warning := range warnings {
			output.PrintWarning(warning)
		}

		// Load production configuration
		prodConfig, err := production.LoadComposeFile(compareProductionPath)
		if err != nil {
			return errors.New(
				errors.ErrCodeInvalidConfig,
				fmt.Sprintf("Failed to load production config from %s", compareProductionPath),
			).WithError(err).WithContext("production_path", compareProductionPath)
		}

		// Compare configurations
		result := production.CompareConfigs(deps, prodConfig)

		// Output results
		if compareJSONOutput {
			encoder := json.NewEncoder(os.Stdout)
			encoder.SetIndent("", "  ")
			if err := encoder.Encode(result); err != nil {
				return errors.New(
					errors.ErrCodeInvalidConfig,
					"Failed to encode JSON output",
				).WithError(err)
			}
		} else {
			output := production.FormatComparisonResult(result)
			fmt.Print(output)
		}

		// Exit with error code if there are critical differences
		hasErrors := false
		for _, diff := range result.ServiceDifferences {
			if diff.Severity == "error" || diff.DependsMismatch != nil {
				hasErrors = true
				break
			}
		}

		if hasErrors {
			return errors.New(
				errors.ErrCodeCompatibilityError,
				"Critical differences found between local and production configurations",
			).WithSuggestion(
				"Review the differences above and update your .raioz.json to match production",
			)
		}

		return nil
	},
}

func init() {
	compareCmd.Flags().StringVarP(
		&configPath,
		"config",
		"c",
		".raioz.json",
		"Path to local .raioz.json",
	)
	compareCmd.Flags().StringVarP(
		&compareProductionPath,
		"production",
		"p",
		"",
		"Path to production docker-compose.yml (required)",
	)
	compareCmd.Flags().BoolVar(
		&compareJSONOutput,
		"json",
		false,
		"Output results in JSON format",
	)
	compareCmd.MarkFlagRequired("production")
	// Note: compareCmd is added to rootCmd in root.go init() to avoid circular dependencies
}
