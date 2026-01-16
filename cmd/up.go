package cmd

import (
	"context"

	"raioz/internal/app"
	"raioz/internal/errors"

	"github.com/spf13/cobra"
)

var (
	configPath   string
	profile      string
	forceReclone bool
	dryRun       bool
)

var upCmd = &cobra.Command{
	Use:          "up",
	Short:        "Bring up project dependencies",
	SilenceUsage: true, // Don't show usage/help on execution errors
	Long: `Bring up all services and infrastructure for the project defined in .raioz.json.

This command will:
1. Validate the configuration
2. Execute preflight checks (Docker, Git, disk space)
3. Resolve workspace and acquire lock
4. Clone/update Git repositories
5. Resolve environment variables
6. Validate and download Docker images
7. Create networks and volumes
8. Generate docker-compose.generated.yml
9. Start services with Docker Compose
10. Save the project state`,
	RunE: func(cmd *cobra.Command, args []string) (err error) {
		// Recover from panics in critical operation
		defer func() {
			if panicErr := errors.RecoverPanic("raioz up"); panicErr != nil {
				err = panicErr
			}
		}()

		// Create context for the operation
		ctx := cmd.Context()
		if ctx == nil {
			ctx = context.Background()
		}

		// Initialize dependencies and use case
		deps := app.NewDependencies()
		upUseCase := app.NewUpUseCase(deps)

		// Execute use case
		return upUseCase.Execute(ctx, app.UpOptions{
			ConfigPath:   configPath,
			Profile:      profile,
			ForceReclone: forceReclone,
			DryRun:       dryRun,
		})
	},
}

func init() {
	upCmd.Flags().StringVarP(&configPath, "config", "c", ".raioz.json", "Path to .raioz.json")
	upCmd.Flags().StringVarP(&profile, "profile", "p", "", "Profile to use (frontend/backend)")
	upCmd.Flags().BoolVar(&forceReclone, "force-reclone", false, "Force re-clone of all git repositories")
	upCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be done without making changes (dependency assist only)")
}
