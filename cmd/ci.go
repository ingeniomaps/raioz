package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"raioz/internal/app"

	"github.com/spf13/cobra"
)

var (
	ciKeep         bool
	ciEphemeral    bool
	ciJobID        string
	ciSkipBuild    bool
	ciSkipPull     bool
	ciOnlyValidate bool
	ciForceReclone bool
)

var ciCmd = &cobra.Command{
	Use:   "ci",
	Short: "CI-optimized command for continuous integration",
	Long: `Optimized command for CI/CD pipelines with:
- Fast validations
- JSON output format
- Ephemeral environments support
- Automatic cleanup

This command is designed to be used in CI/CD pipelines where speed and
parseable output are critical.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		configPath = ResolveConfigPath(configPath)

		deps := app.NewDependencies()
		ciUseCase := app.NewCIUseCase(deps)

		result, err := ciUseCase.Execute(app.CIOptions{
			ConfigPath:   configPath,
			Keep:         ciKeep,
			Ephemeral:    ciEphemeral,
			JobID:        ciJobID,
			SkipBuild:    ciSkipBuild,
			SkipPull:     ciSkipPull,
			OnlyValidate: ciOnlyValidate,
			ForceReclone: ciForceReclone,
		})
		if err != nil {
			return err
		}

		// Output JSON result
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if encErr := encoder.Encode(result); encErr != nil {
			fmt.Fprintf(os.Stderr, "Failed to encode result: %v\n", encErr)
		}

		// Exit with appropriate code
		if !result.Success {
			os.Exit(1)
		}

		return nil
	},
}

func init() {
	ciCmd.Flags().StringVarP(&configPath, "file", "f", ".raioz.json", "Path to config file")
	ciCmd.Flags().BoolVar(&ciKeep, "keep", false, "Keep ephemeral environment after CI run (for debugging)")
	ciCmd.Flags().BoolVar(&ciEphemeral, "ephemeral", false, "Use ephemeral environment (auto-cleanup)")
	ciCmd.Flags().StringVar(&ciJobID, "job-id", "", "CI job ID for ephemeral environment naming")
	ciCmd.Flags().BoolVar(&ciSkipBuild, "skip-build", false, "Skip building and starting services (validation only)")
	ciCmd.Flags().BoolVar(&ciSkipPull, "skip-pull", false, "Skip pulling Docker images")
	ciCmd.Flags().BoolVar(&ciOnlyValidate, "only-validate", false, "Only run validations, skip all setup")
	ciCmd.Flags().BoolVar(&ciForceReclone, "force-reclone", false, "Force re-clone of all git repositories")
	// Note: ciCmd is added to rootCmd in root.go init() to avoid circular dependencies
}
