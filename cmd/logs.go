package cmd

import (
	"context"
	"fmt"

	"raioz/internal/config"
	"raioz/internal/docker"
	"raioz/internal/state"
	"raioz/internal/workspace"

	"github.com/spf13/cobra"
)

var (
	logsFollow bool
	logsTail   int
	logsAll    bool
)

var logsCmd = &cobra.Command{
	Use:   "logs [service...]",
	Short: "View logs for services",
	Long: `View logs for one or more services.

If no service is specified and --all is not set, shows logs for all services.
Use --all to explicitly show logs for all services.
Use --follow to follow logs in real-time.
Use --tail N to show only the last N lines.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Create context for the operation
		ctx := cmd.Context()
		if ctx == nil {
			ctx = context.Background()
		}

		var ws *workspace.Workspace
		var err error

		// Try to determine project name
		if projectName == "" {
			deps, _, _ := config.LoadDeps(configPath)
			if deps != nil {
				projectName = deps.Project.Name
			} else {
				return fmt.Errorf(
					"could not determine project name. "+
						"Please provide --config or --project flag",
				)
			}
		}

		ws, err = workspace.Resolve(projectName)
		if err != nil {
			return fmt.Errorf("failed to resolve workspace: %w", err)
		}

		// Check if state exists
		if !state.Exists(ws) {
			return fmt.Errorf("project is not running (no state file found)")
		}

		composePath := workspace.GetComposePath(ws)

		// Determine which services to show logs for
		var services []string

		if logsAll {
			// Get all services
			allServices, err := docker.GetAvailableServicesWithContext(ctx, composePath)
			if err != nil {
				return fmt.Errorf("failed to get available services: %w", err)
			}
			services = allServices
		} else if len(args) > 0 {
			// Services specified as arguments
			services = args
		} else {
			// No services specified, show all by default
			allServices, err := docker.GetAvailableServicesWithContext(ctx, composePath)
			if err != nil {
				return fmt.Errorf("failed to get available services: %w", err)
			}
			services = allServices
		}

		// Build options
		opts := docker.LogsOptions{
			Follow:   logsFollow,
			Tail:     logsTail,
			Services: services,
		}

		// View logs
		if err := docker.ViewLogsWithContext(ctx, composePath, opts); err != nil {
			return fmt.Errorf("failed to view logs: %w", err)
		}

		return nil
	},
}

func init() {
	logsCmd.Flags().StringVarP(&configPath, "config", "c", ".raioz.json", "Path to .raioz.json")
	logsCmd.Flags().StringVarP(&projectName, "project", "p", "", "Project name (alternative to --config)")
	logsCmd.Flags().BoolVarP(&logsFollow, "follow", "f", false, "Follow log output")
	logsCmd.Flags().IntVar(&logsTail, "tail", 0, "Number of lines to show from the end of logs (0 = all)")
	logsCmd.Flags().BoolVar(&logsAll, "all", false, "Show logs for all services")
}
