package cmd

import (
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
		return executeCICommand()
	},
}

func init() {
	ciCmd.Flags().StringVarP(&configPath, "config", "c", ".raioz.json", "Path to .raioz.json")
	ciCmd.Flags().BoolVar(&ciKeep, "keep", false, "Keep ephemeral environment after CI run (for debugging)")
	ciCmd.Flags().BoolVar(&ciEphemeral, "ephemeral", false, "Use ephemeral environment (auto-cleanup)")
	ciCmd.Flags().StringVar(&ciJobID, "job-id", "", "CI job ID for ephemeral environment naming")
	ciCmd.Flags().BoolVar(&ciSkipBuild, "skip-build", false, "Skip building and starting services (validation only)")
	ciCmd.Flags().BoolVar(&ciSkipPull, "skip-pull", false, "Skip pulling Docker images")
	ciCmd.Flags().BoolVar(&ciOnlyValidate, "only-validate", false, "Only run validations, skip all setup")
	ciCmd.Flags().BoolVar(&ciForceReclone, "force-reclone", false, "Force re-clone of all git repositories")
	// Note: ciCmd is added to rootCmd in root.go init() to avoid circular dependencies
}
