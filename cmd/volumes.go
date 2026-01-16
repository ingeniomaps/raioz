package cmd

import (
	"context"

	"raioz/internal/app"

	"github.com/spf13/cobra"
)

var volumesCmd = &cobra.Command{
	Use:   "volumes",
	Short: "Remove volumes for a project",
	Long: `Remove Docker volumes associated with a project.

This command allows you to clean up volumes after stopping a project.
It will:
1. Identify all volumes used by the project
2. Check if volumes are in use by other projects
3. Ask for confirmation before removing volumes
4. Only remove volumes that are not in use by other projects

Example:
  raioz volumes
  raioz volumes --project myproject
  raioz volumes --config /path/to/.raioz.json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		if ctx == nil {
			ctx = context.Background()
		}

		configPath, _ := cmd.Flags().GetString("config")
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
	volumesCmd.Flags().StringP("config", "c", "", "Path to .raioz.json file")
	volumesCmd.Flags().StringP("project", "p", "", "Project name")
	volumesCmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt and remove all volumes")
	rootCmd.AddCommand(volumesCmd)
}
