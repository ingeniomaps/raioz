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
	watch        bool
	exclusive    bool
	notifyDone   bool
)

var upCmd = &cobra.Command{
	Use:   "up [service...]",
	Short: "Bring up project dependencies",
	Long: "Bring up all services and infrastructure for the project. Pass service " +
		"or dependency names as positional args to start only that subset (with " +
		"their dependsOn ancestors); equivalent to --only and symmetric with " +
		"`raioz down [service...]` / `raioz restart [service...]`.",
	Args:         cobra.ArbitraryArgs,
	SilenceUsage: true, // Don't show usage/help on execution errors
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

		// Issue 011: if the config is a meta-orchestrator, delegate to the
		// MetaRunner before initializing project-mode dependencies.
		if handled, metaErr := tryHandleMeta(ctx, configPath, "up", nil); handled {
			return metaErr
		}

		// Initialize dependencies and use case
		deps := app.NewDependencies()
		upUseCase := app.NewUpUseCase(deps)

		// Positional args fold into --only so `raioz up api web` works the
		// same as `raioz up --only api,web`. Symmetric with down/restart.
		// Mutual conflict (both args + --only with different sets) is
		// resolved by union — the user's intent is "start at least these".
		only := mergeOnlyArgs(args, onlyServices)

		// Execute use case
		execErr := upUseCase.Execute(ctx, app.UpOptions{
			ConfigPath:   configPath,
			Profile:      profile,
			ForceReclone: forceReclone,
			DryRun:       dryRun,
			Only:         only,
			Host:         hostBind,
			Attach:       attach,
			Watch:        watch,
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

// mergeOnlyArgs unions positional args with the --only flag value,
// dropping duplicates while preserving the first-seen order. Either
// list may be empty; an empty result means "start everything".
func mergeOnlyArgs(args, flag []string) []string {
	if len(args) == 0 && len(flag) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(args)+len(flag))
	out := make([]string, 0, len(args)+len(flag))
	for _, list := range [][]string{args, flag} {
		for _, n := range list {
			if n == "" {
				continue
			}
			if _, dup := seen[n]; dup {
				continue
			}
			seen[n] = struct{}{}
			out = append(out, n)
		}
	}
	return out
}

func init() {
	upCmd.Flags().StringVarP(&configPath, "file", "f", "", "Path to config file (auto-detects if omitted)")
	upCmd.Flags().StringVarP(&profile, "profile", "p", "", "Profile to use (frontend/backend)")
	upCmd.Flags().BoolVar(&forceReclone, "force-reclone", false, "Force re-clone of all git repositories")
	upCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be done without making changes")
	upCmd.Flags().StringSliceVar(&onlyServices, "only", nil, "Start only these services (with their dependencies)")
	upCmd.Flags().StringVar(&hostBind, "host", "", "Bind address for shared dev server (e.g., 0.0.0.0)")
	upCmd.Flags().BoolVar(&attach, "attach", false, "Stay attached and stream logs (blocks until Ctrl+C)")
	upCmd.Flags().BoolVar(&watch, "watch", false,
		"File-watch services with watch: true and auto-restart (blocks until Ctrl+C)")
	upCmd.Flags().BoolVar(&exclusive, "exclusive", false, i18n.T("cmd.up.flag.exclusive"))
	upCmd.Flags().BoolVar(&notifyDone, "notify", false, i18n.T("cmd.up.flag.notify"))
}
