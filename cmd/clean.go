package cmd

import (
	"context"

	"raioz/internal/app"

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
	Long:  "Clean up stopped services and unused Docker resources.",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		if ctx == nil {
			ctx = context.Background()
		}

		configPath = ResolveConfigPath(configPath)

		deps := app.NewDependencies()
		cleanUseCase := app.NewCleanUseCase(deps)

		return cleanUseCase.Execute(ctx, app.CleanOptions{
			ConfigPath:  configPath,
			ProjectName: projectName,
			All:         cleanAll,
			Images:      cleanImages,
			Volumes:     cleanVolumes,
			Networks:    cleanNetworks,
			DryRun:      cleanDryRun,
			Force:       cleanForce,
		})
	},
}

func init() {
	cleanCmd.Flags().StringVarP(&configPath, "file", "f", ".raioz.json", "Path to config file")
	cleanCmd.Flags().StringVarP(&projectName, "project", "p", "", "Project name (alternative to --file)")
	cleanCmd.Flags().BoolVar(&cleanAll, "all", false, "Clean all projects")
	cleanCmd.Flags().BoolVar(&cleanImages, "images", false, "Remove unused Docker images")
	cleanCmd.Flags().BoolVar(&cleanVolumes, "volumes", false, "Remove unused Docker volumes (requires confirmation)")
	cleanCmd.Flags().BoolVar(&cleanNetworks, "networks", false, "Remove unused Docker networks")
	cleanCmd.Flags().BoolVar(&cleanDryRun, "dry-run", false, "Show what would be done without making changes")
	cleanCmd.Flags().BoolVar(&cleanForce, "force", false, "Skip confirmation prompts (use with caution)")
}
