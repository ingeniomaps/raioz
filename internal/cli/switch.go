package cli

import (
	"context"
	"time"

	"raioz/internal/app"
	"raioz/internal/errors"
	"raioz/internal/logging"

	"github.com/spf13/cobra"
)

var (
	switchYes  bool
	switchKeep string
)

var switchCmd = &cobra.Command{
	Use:   "switch",
	Short: "Free colliding host ports and bring up the cwd project",
	Long: "Detects which active raioz projects (cross-workspace) hold host " +
		"ports declared in the cwd's raioz.yaml, prompts to confirm, stops " +
		"them, and then runs `raioz up`. Combines `down --conflicting` + " +
		"`up` into one step.",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) (err error) {
		defer func() {
			if panicErr := errors.RecoverPanic("raioz switch"); panicErr != nil {
				err = panicErr
			}
		}()

		startTime := time.Now()

		ctx := cmd.Context()
		if ctx == nil {
			ctx = context.Background()
		}
		ctx = logging.WithRequestID(ctx)
		ctx = logging.WithOperation(ctx, "raioz switch")

		configPath = ResolveConfigPath(configPath)

		deps := newDependencies()
		uc := app.NewSwitchUseCase(deps)

		switchErr := uc.Execute(ctx, app.SwitchOptions{
			ConfigPath: configPath,
			Yes:        switchYes,
			Keep:       app.SplitKeepList(switchKeep),
		})

		logging.LogOperationEnd(ctx, "raioz switch", startTime, switchErr)
		return switchErr
	},
}

func init() {
	switchCmd.Flags().StringVarP(&configPath, "file", "f", "",
		"Path to config file")
	switchCmd.Flags().BoolVarP(&switchYes, "yes", "y", false,
		"Skip the confirmation prompt")
	switchCmd.Flags().StringVar(&switchKeep, "keep", "",
		"Comma-separated project names to spare from teardown")
}
