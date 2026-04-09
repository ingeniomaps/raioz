package cli

import (
	"raioz/internal/app"

	"github.com/spf13/cobra"
)

var devReset bool
var devList bool

var devCmd = &cobra.Command{
	Use:          "dev [dependency] [local-path]",
	Short:        "Promote a dependency to local development",
	Long:         "Switch a dependency from its Docker image to a local path for development. Use --reset to switch back.",
	SilenceUsage: true,
	Args:         cobra.MaximumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		deps := app.NewDependencies()
		useCase := app.NewDevUseCase(deps)

		configPath := ResolveConfigPath(devConfigPath)

		opts := app.DevOptions{
			ConfigPath: configPath,
			List:       devList,
			Reset:      devReset,
		}

		if len(args) >= 1 {
			opts.Name = args[0]
		}
		if len(args) >= 2 {
			opts.LocalPath = args[1]
		}

		return useCase.Execute(ctx, opts)
	},
}

var devConfigPath string

func init() {
	devCmd.Flags().StringVarP(&devConfigPath, "file", "f", "", "Path to config file")
	devCmd.Flags().BoolVar(&devReset, "reset", false, "Reset dependency back to its Docker image")
	devCmd.Flags().BoolVar(&devList, "list", false, "List active dev overrides")
}
