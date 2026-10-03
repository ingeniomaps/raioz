package cli

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
	Short: "Validate the project and pull its images, with JSON output",
	Long:  "Validate raioz.yaml and pull the dependency images. Starts nothing; prints a JSON report.",
	RunE: func(cmd *cobra.Command, args []string) error {
		configPath := ResolveConfigPath(configPath)

		deps := newDependencies()
		ciUseCase := app.NewCIUseCase(deps)

		result, err := ciUseCase.Execute(app.CIOptions{
			ConfigPath:   configPath,
			SkipPull:     ciSkipPull,
			OnlyValidate: ciOnlyValidate,
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
	ciCmd.Flags().StringVarP(&configPath, "file", "f", "", "Path to config file")
	ciCmd.Flags().BoolVar(&ciKeep, "keep", false, "Keep ephemeral environment after CI run (for debugging)")
	ciCmd.Flags().BoolVar(&ciEphemeral, "ephemeral", false, "Use ephemeral environment (auto-cleanup)")
	ciCmd.Flags().StringVar(&ciJobID, "job-id", "", "CI job ID for ephemeral environment naming")
	ciCmd.Flags().BoolVar(&ciSkipBuild, "skip-build", false, "Skip building and starting services (validation only)")
	ciCmd.Flags().BoolVar(&ciSkipPull, "skip-pull", false, "Skip pulling Docker images")
	ciCmd.Flags().BoolVar(&ciOnlyValidate, "only-validate", false, "Only run validations, skip all setup")
	ciCmd.Flags().BoolVar(&ciForceReclone, "force-reclone", false, "Force re-clone of all git repositories")
	// ci validates and pulls; it never started anything. These flags
	// promised an ephemeral environment that was not built — they stay
	// accepted so a pipeline that passes them keeps working, and say so.
	for _, name := range []string{"keep", "ephemeral", "job-id", "skip-build", "force-reclone"} {
		_ = ciCmd.Flags().MarkDeprecated(name, "it has no effect: ci validates and pulls images, it does not start anything")
	}
	// Note: ciCmd is added to rootCmd in root.go init() to avoid circular dependencies
}
