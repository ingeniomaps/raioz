package cli

import (
	"context"

	"raioz/internal/app"
	"raioz/internal/errors"
	"raioz/internal/i18n"
	"raioz/internal/notify"

	"github.com/spf13/cobra"
)

var (
	configPath   string
	profile      string
	forceReclone bool
	dryRun       bool
	onlyServices []string
	hostBind     string
	attach       bool
	exclusive    bool
	notifyDone   bool
)

var upCmd = &cobra.Command{
	Use:          "up",
	Short:        "Bring up project dependencies",
	SilenceUsage: true, // Don't show usage/help on execution errors
	Long:         "Bring up all services and infrastructure for the project defined in .raioz.json.",
	RunE: func(cmd *cobra.Command, args []string) (err error) {
		// Recover from panics in critical operation
		defer func() {
			if panicErr := errors.RecoverPanic("raioz up"); panicErr != nil {
				err = panicErr
			}
		}()

		// Create context for the operation
		ctx := cmd.Context()
		if ctx == nil {
			ctx = context.Background()
		}

		// Resolve config path: empty -> .raioz.json; otherwise use given path (any name/ruta)
		configPath = ResolveConfigPath(configPath)

		// Initialize dependencies and use case
		deps := app.NewDependencies()
		upUseCase := app.NewUpUseCase(deps)

		// Execute use case
		execErr := upUseCase.Execute(ctx, app.UpOptions{
			ConfigPath:   configPath,
			Profile:      profile,
			ForceReclone: forceReclone,
			DryRun:       dryRun,
			Only:         onlyServices,
			Host:         hostBind,
			Attach:       attach,
			Exclusive:    exclusive,
		})

		if notifyDone {
			if execErr == nil {
				notify.Send("Raioz", i18n.T("up.notify_ready"))
			} else {
				notify.Send("Raioz", i18n.T("up.notify_failed"))
			}
		}

		return execErr
	},
}

func init() {
	upCmd.Flags().StringVarP(&configPath, "file", "f", "", "Path to config file (auto-detects if omitted)")
	upCmd.Flags().StringVarP(&profile, "profile", "p", "", "Profile to use (frontend/backend)")
	upCmd.Flags().BoolVar(&forceReclone, "force-reclone", false, "Force re-clone of all git repositories")
	upCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be done without making changes")
	upCmd.Flags().StringSliceVar(&onlyServices, "only", nil, "Start only these services (with their dependencies)")
	upCmd.Flags().StringVar(&hostBind, "host", "", "Bind address for shared dev server (e.g., 0.0.0.0)")
	upCmd.Flags().BoolVar(&attach, "attach", false, "Stay attached and stream logs (without file watching)")
	upCmd.Flags().BoolVar(&exclusive, "exclusive", false, i18n.T("cmd.up.flag.exclusive"))
	upCmd.Flags().BoolVar(&notifyDone, "notify", false, i18n.T("cmd.up.flag.notify"))
}
