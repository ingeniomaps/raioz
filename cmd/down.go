package cmd

import (
	"context"
	"time"

	"raioz/internal/app"
	"raioz/internal/config"
	"raioz/internal/errors"
	"raioz/internal/logging"
	"raioz/internal/output"

	"github.com/spf13/cobra"
)

var projectName string

	var downCmd = &cobra.Command{
	Use:          "down",
	Short:        "Bring down project dependencies",
	SilenceUsage: true, // Don't show usage/help on execution errors
	RunE: func(cmd *cobra.Command, args []string) (err error) {
		// Recover from panics in critical operation
		defer func() {
			if panicErr := errors.RecoverPanic("raioz down"); panicErr != nil {
				err = panicErr
			}
		}()

		startTime := time.Now()

		// Create context for the entire operation
		ctx := cmd.Context()
		if ctx == nil {
			ctx = context.Background()
		}

		// Add request ID and operation context for logging correlation
		ctx = logging.WithRequestID(ctx)
		ctx = logging.WithOperation(ctx, "raioz down")

		// Initialize dependencies and use case
		deps := app.NewDependencies()
		downUseCase := app.NewDownUseCase(deps)

		// Load config deps first to check if this is a local project with commands
		configDeps, _, loadErr := config.LoadDeps(configPath)
		isLocalProjectWithCommands := false
		var projectDir string
		if loadErr == nil && configDeps != nil {
			isLocal, dir, checkErr := isLocalProject(configPath)
			if checkErr == nil && isLocal {
				isLocalProjectWithCommands = true
				projectDir = dir
			}
		}

		// Execute use case
		err = downUseCase.Execute(ctx, app.DownOptions{
			ProjectName: projectName,
			ConfigPath:  configPath,
		})

		// Execute local project down command if this is a local project
		// Execute even if use case failed (e.g., no state file) if project has commands
		if isLocalProjectWithCommands && configDeps != nil {
			// Determine mode
			mode := "dev"
			for _, svc := range configDeps.Services {
				if svc.Docker != nil && svc.Docker.Mode != "" {
					mode = svc.Docker.Mode
					break
				}
			}

			// Get down command first to check if it exists
			downCommand := getLocalProjectCommand(configDeps, "down", mode)
			if downCommand != "" {
				// Check health before down
				healthCommand := getLocalProjectCommand(configDeps, "health", mode)
				if healthCommand != "" {
					isHealthy, healthErr := checkLocalProjectHealth(ctx, projectDir, healthCommand)
					if healthErr == nil {
						if !isHealthy {
							// Project is not healthy (not running), nothing to stop
							logging.InfoWithContext(ctx, "Project is not healthy, skipping down command")
							output.PrintInfo("ℹ️  Project is not running, nothing to stop")
							// Return nil instead of err to avoid showing error when project is just not running
							return nil
						}
						// Project is healthy (running), proceed with down
					}
				}

				// If use case failed due to missing state but we have a local command, show a clearer message
				if err != nil {
					// Check if error is about missing state
					if raiozErr, ok := err.(*errors.RaiozError); ok && raiozErr.Code == errors.ErrCodeStateLoadError {
						output.PrintInfo("ℹ️  No raioz state found, but executing local project down command...")
					} else {
						output.PrintInfo("ℹ️  Executing local project down command...")
					}
				} else {
					output.PrintInfo("ℹ️  Executing local project down command...")
				}

				if execErr := executeLocalProjectCommand(ctx, projectDir, downCommand, mode); execErr != nil {
					logging.WarnWithContext(ctx, "Failed to execute local project down command", "error", execErr.Error())
					output.PrintError("✗ Failed to execute local project down command")
					// Return the execution error instead of the state error
					return execErr
				} else {
					output.PrintSuccess("✔ Local project down command executed successfully")
					// If local command executed successfully, return nil even if use case failed
					return nil
				}
			}
		}

		// Log operation completion
		logging.LogOperationEnd(ctx, "raioz down", startTime, err,
			"project", projectName,
		)

		return err
	},
}

func init() {
	downCmd.Flags().StringVarP(&configPath, "config", "c", ".raioz.json", "Path to .raioz.json")
	downCmd.Flags().StringVarP(&projectName, "project", "p", "", "Project name (alternative to --config)")
}
