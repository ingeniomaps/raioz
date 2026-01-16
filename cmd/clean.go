package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"raioz/internal/config"
	"raioz/internal/docker"
	"raioz/internal/logging"
	"raioz/internal/output"
	"raioz/internal/workspace"

	"github.com/spf13/cobra"
)

var (
	cleanAll      bool
	cleanImages   bool
	cleanVolumes  bool
	cleanNetworks bool
	cleanDryRun   bool
	cleanForce    bool
)

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Clean up stopped services and unused resources",
	Long: `Clean up stopped services and unused Docker resources.

By default, cleans stopped services for the current project.
Use flags to clean additional resources or all projects.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Create context for the operation
		ctx := cmd.Context()
		if ctx == nil {
			ctx = context.Background()
		}

		var ws *workspace.Workspace
		var err error

		// Determine project name if not cleaning all
		if !cleanAll {
			if projectName == "" {
				deps, _, _ := config.LoadDeps(configPath)
				if deps != nil {
					projectName = deps.Project.Name
				} else {
					return fmt.Errorf(
						"could not determine project name. "+
							"Please provide --config, --project, or use --all",
					)
				}
			}
		}

		var actions []string

		// Clean projects
		if cleanAll {
			baseDir, err := workspace.GetBaseDir()
			if err != nil {
				return fmt.Errorf("failed to get base directory: %w", err)
			}

			logging.Info("Cleaning all projects")
			projectActions, err := docker.CleanAllProjectsWithContext(ctx, baseDir, cleanDryRun)
			if err != nil {
				return fmt.Errorf("failed to clean all projects: %w", err)
			}
			actions = append(actions, projectActions...)
		} else {
			ws, err = workspace.Resolve(projectName)
			if err != nil {
				return fmt.Errorf("failed to resolve workspace: %w", err)
			}

			composePath := workspace.GetComposePath(ws)
			logging.Info("Cleaning project", "project", projectName)

			projectActions, err := docker.CleanProjectWithContext(ctx, composePath, cleanDryRun)
			if err != nil {
				return fmt.Errorf("failed to clean project: %w", err)
			}
			actions = append(actions, projectActions...)

			// Remove state file if exists
			statePath := workspace.GetStatePath(ws)
			if _, err := os.Stat(statePath); err == nil {
				if cleanDryRun {
					actions = append(actions, fmt.Sprintf("Would remove state file: %s", statePath))
				} else {
					if err := os.Remove(statePath); err != nil {
						actions = append(actions, fmt.Sprintf("⚠️  Failed to remove state file: %v", err))
					} else {
						actions = append(actions, fmt.Sprintf("Removed state file: %s", statePath))
					}
				}
			}
		}

		// Clean images
		if cleanImages {
			logging.Info("Cleaning unused images")
			imageActions, err := docker.CleanUnusedImagesWithContext(ctx, cleanDryRun)
			if err != nil {
				return fmt.Errorf("failed to clean images: %w", err)
			}
			actions = append(actions, imageActions...)
		}

		// Clean volumes
		if cleanVolumes {
			logging.Info("Cleaning unused volumes")

			// Check if force is required
			if !cleanForce && !cleanDryRun {
				logging.Warn("Volume removal requires confirmation. Use --force to proceed.")
				fmt.Print("Are you sure you want to remove unused volumes? (yes/no): ")
				reader := bufio.NewReader(os.Stdin)
				response, err := reader.ReadString('\n')
				if err != nil {
					return fmt.Errorf("failed to read response: %w", err)
				}
				response = strings.TrimSpace(strings.ToLower(response))
				if response != "yes" && response != "y" {
					logging.Info("Volume cleanup cancelled")
					return nil
				}
			}

			volumeActions, err := docker.CleanUnusedVolumesWithContext(ctx, cleanDryRun, cleanForce || cleanDryRun)
			if err != nil {
				return fmt.Errorf("failed to clean volumes: %w", err)
			}
			actions = append(actions, volumeActions...)
		}

		// Clean networks
		if cleanNetworks {
			logging.Info("Cleaning unused networks")
			networkActions, err := docker.CleanUnusedNetworksWithContext(ctx, cleanDryRun)
			if err != nil {
				return fmt.Errorf("failed to clean networks: %w", err)
			}
			actions = append(actions, networkActions...)
		}

		// Display actions
		if cleanDryRun {
			output.PrintSectionHeader("Dry Run - Actions That Would Be Taken")
		} else {
			output.PrintSectionHeader("Actions Taken")
		}

		if len(actions) == 0 {
			output.PrintInfo("Nothing to clean")
		} else {
			output.PrintList(actions, 0)
		}

		return nil
	},
}

func init() {
	cleanCmd.Flags().StringVarP(&configPath, "config", "c", ".raioz.json", "Path to .raioz.json")
	cleanCmd.Flags().StringVarP(&projectName, "project", "p", "", "Project name (alternative to --config)")
	cleanCmd.Flags().BoolVar(&cleanAll, "all", false, "Clean all projects")
	cleanCmd.Flags().BoolVar(&cleanImages, "images", false, "Remove unused Docker images")
	cleanCmd.Flags().BoolVar(&cleanVolumes, "volumes", false, "Remove unused Docker volumes (requires confirmation)")
	cleanCmd.Flags().BoolVar(&cleanNetworks, "networks", false, "Remove unused Docker networks")
	cleanCmd.Flags().BoolVar(&cleanDryRun, "dry-run", false, "Show what would be done without making changes")
	cleanCmd.Flags().BoolVarP(&cleanForce, "force", "f", false, "Skip confirmation prompts (use with caution)")
}
