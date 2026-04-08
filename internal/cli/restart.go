package cli

import (
	"context"

	"raioz/internal/app"
	"raioz/internal/errors"

	"github.com/spf13/cobra"
)

var (
	restartAll           bool
	restartIncludeInfra  bool
	restartForceRecreate bool
)

var restartCmd = &cobra.Command{
	Use:          "restart [service...]",
	Short:        "Restart project services",
	SilenceUsage: true,
	Long:         "Restart one or more services for the current project.",
	RunE: func(cmd *cobra.Command, args []string) (err error) {
		defer func() {
			if panicErr := errors.RecoverPanic("raioz restart"); panicErr != nil {
				err = panicErr
			}
		}()

		ctx := cmd.Context()
		if ctx == nil {
			ctx = context.Background()
		}

		configPath = ResolveConfigPath(configPath)

		deps := app.NewDependencies()
		useCase := app.NewRestartUseCase(deps)

		return useCase.Execute(ctx, app.RestartOptions{
			ConfigPath:    configPath,
			ProjectName:   projectName,
			All:           restartAll,
			IncludeInfra:  restartIncludeInfra,
			ForceRecreate: restartForceRecreate,
			Services:      args,
		})
	},
}

func init() {
	restartCmd.Flags().StringVarP(&configPath, "file", "f", ".raioz.json", "Path to config file")
	restartCmd.Flags().StringVarP(&projectName, "project", "p", "", "Project name (alternative to --file)")
	restartCmd.Flags().BoolVar(&restartAll, "all", false, "Restart all services")
	restartCmd.Flags().BoolVar(&restartIncludeInfra, "include-infra", false, "Also restart infrastructure services")
	restartCmd.Flags().BoolVar(&restartForceRecreate, "force-recreate", false, "Force recreate containers")
}
