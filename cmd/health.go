package cmd

import (
	"context"

	"raioz/internal/config"
	"raioz/internal/errors"
	"raioz/internal/logging"
	"raioz/internal/output"

	"github.com/spf13/cobra"
)

var healthCmd = &cobra.Command{
	Use:          "health",
	Short:        "Check project health and manage local project lifecycle",
	SilenceUsage: true, // Don't show usage/help on execution errors
	Long: `Check if the local project is running and manage its lifecycle.

This command:
1. Checks if the project is running (using health command if defined)
2. If not running, starts it with the 'up' command
3. If running, does nothing
4. If needs to be stopped, stops it with the 'down' command

The health command can be defined in .raioz.json under project.commands.health`,
	RunE: func(cmd *cobra.Command, args []string) (err error) {
		// Recover from panics
		defer func() {
			if panicErr := errors.RecoverPanic("raioz health"); panicErr != nil {
				err = panicErr
			}
		}()

		// Create context
		ctx := cmd.Context()
		if ctx == nil {
			ctx = context.Background()
		}

		ctx = logging.WithRequestID(ctx)
		ctx = logging.WithOperation(ctx, "raioz health")

		// Load configuration
		deps, _, err := config.LoadDeps(configPath)
		if err != nil {
			return errors.New(
				errors.ErrCodeInvalidConfig,
				"Failed to load configuration",
			).WithSuggestion(
				"Ensure .raioz.json exists and is valid JSON.",
			).WithError(err)
		}

		// Check if this is a local project
		isLocal, projectDir, err := isLocalProject(configPath)
		if err != nil {
			return errors.New(
				errors.ErrCodeWorkspaceError,
				"Failed to check if project is local",
			).WithError(err)
		}

		if !isLocal {
			output.PrintInfo("This is not a local project. Health check only applies to local projects.")
			return nil
		}

		// Determine mode
		mode := "dev"
		for _, svc := range deps.Services {
			if svc.Docker != nil && svc.Docker.Mode != "" {
				mode = svc.Docker.Mode
				break
			}
		}

		// Get health command
		healthCommand := getLocalProjectCommand(deps, "health", mode)

		// Check health
		isHealthy, err := checkLocalProjectHealth(ctx, projectDir, healthCommand)
		if err != nil {
			return errors.New(
				errors.ErrCodeWorkspaceError,
				"Failed to check project health",
			).WithError(err)
		}

		if isHealthy {
			output.PrintSuccess("Project is healthy and running")
			return nil
		}

		// Project is not healthy
		output.PrintWarning("Project is not healthy (not running)")
		return nil
	},
}

func init() {
	healthCmd.Flags().StringVarP(&configPath, "config", "c", ".raioz.json", "Path to .raioz.json")
	rootCmd.AddCommand(healthCmd)
}
