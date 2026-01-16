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

		// Try to determine project name and workspace
		var workspaceName string
		if projectName == "" {
			deps, _, _ := config.LoadDeps(configPath)
			if deps != nil {
				projectName = deps.Project.Name
				workspaceName = deps.GetWorkspaceName()
			} else {
				return fmt.Errorf(
					"could not determine project name. "+
						"Please provide --config or --project flag",
				)
			}
		} else {
			// If project name comes from CLI, load config to get workspace name
			deps, _, _ := config.LoadDeps(configPath)
			if deps != nil && deps.Project.Name == projectName {
				workspaceName = deps.GetWorkspaceName()
			} else {
				// Fallback: use project name as workspace (backward compatibility)
				workspaceName = projectName
			}
		}

		ws, err = workspace.Resolve(workspaceName)
		if err != nil {
			return fmt.Errorf("failed to resolve workspace: %w", err)
		}

		// Check if state exists
		if !state.Exists(ws) {
			return fmt.Errorf("project is not running (no state file found)")
		}

		// Load state to check for project compose path
		stateDeps, err := state.Load(ws)
		if err != nil {
			return fmt.Errorf("failed to load project state: %w", err)
		}

		composePath := workspace.GetComposePath(ws)

		// Determine which services to show logs for
		var services []string
		var projectComposeServices []string

		if logsAll {
			// Get all services from generated compose
			allServices, err := docker.GetAvailableServicesWithContext(ctx, composePath)
			if err != nil {
				return fmt.Errorf("failed to get available services: %w", err)
			}
			services = allServices

			// Get all services from project compose if it exists
			if stateDeps != nil && stateDeps.ProjectComposePath != "" {
				projectServices, err := docker.GetAvailableServicesWithContext(ctx, stateDeps.ProjectComposePath)
				if err == nil {
					projectComposeServices = projectServices
				}
			}
		} else if len(args) > 0 {
			// Services specified as arguments - separate them between generated and project compose
			// First, get all services from both compose files
			generatedServices, _ := docker.GetAvailableServicesWithContext(ctx, composePath)
			if stateDeps != nil && stateDeps.ProjectComposePath != "" {
				projectServices, err := docker.GetAvailableServicesWithContext(ctx, stateDeps.ProjectComposePath)
				if err == nil {
					// Separate args: which belong to generated compose, which to project compose
					for _, arg := range args {
						foundInGenerated := false
						for _, genSvc := range generatedServices {
							if arg == genSvc {
								services = append(services, arg)
								foundInGenerated = true
								break
							}
						}
						if !foundInGenerated {
							// Check if it's in project compose
							for _, projSvc := range projectServices {
								if arg == projSvc {
									projectComposeServices = append(projectComposeServices, arg)
									break
								}
							}
						}
					}
				} else {
					// If can't get project services, assume all args are for generated compose
					services = args
				}
			} else {
				// No project compose, all args are for generated compose
				services = args
			}
		} else {
			// No services specified, show all by default
			allServices, err := docker.GetAvailableServicesWithContext(ctx, composePath)
			if err != nil {
				return fmt.Errorf("failed to get available services: %w", err)
			}
			services = allServices

			// Get all services from project compose if it exists
			if stateDeps != nil && stateDeps.ProjectComposePath != "" {
				projectServices, err := docker.GetAvailableServicesWithContext(ctx, stateDeps.ProjectComposePath)
				if err == nil {
					projectComposeServices = projectServices
				}
			}
		}

		// View logs from generated compose
		if len(services) > 0 {
			opts := docker.LogsOptions{
				Follow:   logsFollow,
				Tail:     logsTail,
				Services: services,
			}
			if err := docker.ViewLogsWithContext(ctx, composePath, opts); err != nil {
				return fmt.Errorf("failed to view logs: %w", err)
			}
		}

		// View logs from project compose if it exists and has services
		if len(projectComposeServices) > 0 && stateDeps != nil && stateDeps.ProjectComposePath != "" {
			opts := docker.LogsOptions{
				Follow:   logsFollow,
				Tail:     logsTail,
				Services: projectComposeServices,
			}
			if err := docker.ViewLogsWithContext(ctx, stateDeps.ProjectComposePath, opts); err != nil {
				return fmt.Errorf("failed to view project compose logs: %w", err)
			}
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
