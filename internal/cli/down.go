package cli

import (
	"context"
	"time"

	"raioz/internal/app"
	"raioz/internal/errors"
	"raioz/internal/logging"

	"github.com/spf13/cobra"
)

var projectName string
var downAll bool
var pruneShared bool
var downConflicting bool
var downAllProjects bool

var downCmd = &cobra.Command{
	Use:          "down",
	Short:        "Bring down project dependencies",
	Long:         "Bring down all services and infrastructure for the current project.",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) (err error) {
		defer func() {
			if panicErr := errors.RecoverPanic("raioz down"); panicErr != nil {
				err = panicErr
			}
		}()

		startTime := time.Now()

		ctx := cmd.Context()
		if ctx == nil {
			ctx = context.Background()
		}

		ctx = logging.WithRequestID(ctx)
		ctx = logging.WithOperation(ctx, "raioz down")

		configPath = ResolveConfigPath(configPath)

		deps := app.NewDependencies()
		downUseCase := app.NewDownUseCase(deps)

		if downConflicting && downAllProjects {
			return errors.New(
				errors.ErrCodeInvalidConfig,
				"--conflicting and --all-projects are mutually exclusive",
			).WithSuggestion(
				"Pick one: --conflicting frees host ports; --all-projects " +
					"stops every other active project regardless of ports.",
			)
		}

		downErr := downUseCase.Execute(ctx, app.DownOptions{
			ProjectName: projectName,
			ConfigPath:  configPath,
			All:         downAll,
			PruneShared: pruneShared,
			Conflicting: downConflicting,
			AllProjects: downAllProjects,
		})

		// Handle local project down command
		baseDir, _ := deps.Workspace.GetBaseDir()
		if baseDir != "" {
			handled, localErr := app.HandleLocalProjectDown(ctx, configPath, baseDir, downErr)
			if handled {
				return localErr
			}
		}

		logging.LogOperationEnd(ctx, "raioz down", startTime, downErr, "project", projectName)

		return downErr
	},
}

func init() {
	downCmd.Flags().StringVarP(&configPath, "file", "f", "", "Path to config file")
	downCmd.Flags().StringVarP(&projectName, "project", "p", "", "Project name (alternative to --file)")
	downCmd.Flags().BoolVar(&downAll, "all", false, "Stop all workspace services and infra (full shutdown)")
	downCmd.Flags().BoolVar(&pruneShared, "prune-shared", false, "Also stop infra if no other active projects use it")
	downCmd.Flags().BoolVar(&downConflicting, "conflicting", false,
		"Stop other active raioz projects whose host ports collide with the cwd's raioz.yaml")
	downCmd.Flags().BoolVar(&downAllProjects, "all-projects", false,
		"Stop every active raioz project except the cwd's (cross-workspace)")
}
