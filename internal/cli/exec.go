package cli

import (
	"context"

	"raioz/internal/app"
	"raioz/internal/errors"

	"github.com/spf13/cobra"
)

var execInteractive bool

var execCmd = &cobra.Command{
	Use:          "exec <service> [command...]",
	Short:        "Run a command inside a running service container",
	SilenceUsage: true,
	Long:         "Run a command inside a running service container.\n\nIf no command is specified, opens a shell (sh).",
	Args:         cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) (err error) {
		defer func() {
			if panicErr := errors.RecoverPanic("raioz exec"); panicErr != nil {
				err = panicErr
			}
		}()

		ctx := cmd.Context()
		if ctx == nil {
			ctx = context.Background()
		}

		configPath = ResolveConfigPath(configPath)

		deps := app.NewDependencies()
		useCase := app.NewExecUseCase(deps)

		serviceName := args[0]
		command := args[1:]

		return useCase.Execute(ctx, app.ExecOptions{
			ConfigPath:  configPath,
			ProjectName: projectName,
			Service:     serviceName,
			Command:     command,
			Interactive: execInteractive,
		})
	},
}

func init() {
	execCmd.Flags().StringVarP(&configPath, "file", "f", ".raioz.json", "Path to config file")
	execCmd.Flags().StringVarP(&projectName, "project", "p", "", "Project name (alternative to --file)")
	execCmd.Flags().BoolVarP(&execInteractive, "interactive", "i", true, "Keep stdin open and allocate TTY")
	execCmd.Flags().SetInterspersed(false)
}
