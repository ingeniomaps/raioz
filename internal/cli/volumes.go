package cli

import (
	"context"

	"raioz/internal/app"

	"github.com/spf13/cobra"
)

var volumesCmd = &cobra.Command{
	Use:   "volumes",
	Short: "Manage project volumes",
	Long:  "Manage Docker volumes associated with a project.",
}

var volumesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List project volumes",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		if ctx == nil {
			ctx = context.Background()
		}

		configPath = ResolveConfigPath(configPath)
		deps := app.NewDependencies()
		useCase := app.NewVolumesUseCase(deps)

		return useCase.List(ctx, app.VolumesOptions{
			ConfigPath:  configPath,
			ProjectName: projectName,
		})
	},
}

var volumesRemoveCmd = &cobra.Command{
	Use:     "remove [volume...]",
	Aliases: []string{"rm"},
	Short:   "Remove project volumes",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		if ctx == nil {
			ctx = context.Background()
		}

		configPath = ResolveConfigPath(configPath)
		all, _ := cmd.Flags().GetBool("all")
		force, _ := cmd.Flags().GetBool("force")

		deps := app.NewDependencies()
		useCase := app.NewVolumesUseCase(deps)

		return useCase.Remove(ctx, app.VolumesRemoveOptions{
			ConfigPath:  configPath,
			ProjectName: projectName,
			All:         all,
			Force:       force,
			Volumes:     args,
		})
	},
}

func init() {
	volumesCmd.PersistentFlags().StringVarP(&configPath, "file", "f", ".raioz.json", "Path to config file")
	volumesCmd.PersistentFlags().StringVarP(&projectName, "project", "p", "", "Project name")

	volumesRemoveCmd.Flags().Bool("all", false, "Remove all project volumes")
	volumesRemoveCmd.Flags().Bool("force", false, "Skip confirmation prompt")

	volumesCmd.AddCommand(volumesListCmd)
	volumesCmd.AddCommand(volumesRemoveCmd)
}
