package cmd

import (
	"context"

	"raioz/internal/app"

	"github.com/spf13/cobra"
)

var volumesCmd = &cobra.Command{
	Use:   "volumes",
	Short: "Remove volumes for a project",
	Long:  "Remove Docker volumes associated with a project.",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		if ctx == nil {
			ctx = context.Background()
		}

		configPath, _ := cmd.Flags().GetString("file")
		projectName, _ := cmd.Flags().GetString("project")
		force, _ := cmd.Flags().GetBool("force")

		// Default config path
		if configPath == "" {
			configPath = ".raioz.json"
		}

		deps := app.NewDependencies()
		useCase := app.NewVolumesUseCase(deps)

		opts := app.VolumesOptions{
			ConfigPath:  configPath,
			ProjectName: projectName,
			Force:       force,
		}

		if err := useCase.Execute(ctx, opts); err != nil {
			return err
		}

		return nil
	},
}

func init() {
	volumesCmd.Flags().StringP("file", "f", "", "Path to config file")
	volumesCmd.Flags().StringP("project", "p", "", "Project name")
	volumesCmd.Flags().Bool("force", false, "Skip confirmation prompt and remove all volumes")
	rootCmd.AddCommand(volumesCmd)
}
