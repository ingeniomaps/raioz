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
	configPath    string
	profile       string
	metaProfiles  []string
	forceReclone  bool
	dryRun        bool
	onlyServices  []string
	hostBind      string
	attach        bool
	watch         bool
	exclusive     bool
	notifyDone    bool
	routerOff     bool
	auditSiblings bool
	noClone       bool
	forceRemote   []string
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

		// If the config is a meta-orchestrator, delegate to the
		// MetaRunner before initializing project-mode dependencies.
		if handled, metaErr := tryHandleMeta(
			ctx, configPath, "up", nil, metaProfiles,
			app.MetaUpOptions{
				RouterOff:     routerOff,
				AuditSiblings: auditSiblings,
				NoClone:       noClone,
				ForceRemote:   forceRemote,
			},
		); handled {
			if notifyDone {
				if metaErr == nil {
					notify.Send("Raioz", i18n.T("up.notify_ready"))
				} else {
					notify.Send("Raioz", i18n.T("up.notify_failed"))
				}
			}
			return metaErr
		}

		// Initialize dependencies and use case
		deps := newDependencies()
		upUseCase := app.NewUpUseCase(deps)

		// Positional args fold into --only so `raioz up api web` works the
		// same as `raioz up --only api,web`. Symmetric with down/restart.
		// Mutual conflict (both args + --only with different sets) is
		// resolved by union — the user's intent is "start at least these".
		only := mergeOnlyArgs(args, onlyServices)

		// Execute use case
		execErr := upUseCase.Execute(ctx, app.UpOptions{
			ConfigPath:    configPath,
			Profile:       profile,
			ForceReclone:  forceReclone,
			DryRun:        dryRun,
			Only:          only,
			Host:          hostBind,
			Attach:        attach,
			Watch:         watch,
			Exclusive:     exclusive,
			RouterOff:     routerOff,
			AuditSiblings: auditSiblings,
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
	upCmd.Flags().StringSliceVar(&metaProfiles, "meta-profile", nil,
		"Activate meta sub-projects tagged with these profiles (kind: meta only). Repeatable.")
	upCmd.Flags().BoolVar(&auditSiblings, "audit-siblings", false,
		"Run ADR-036 hygiene gates (H1 secret scan, H2 path safety, H3 image "+
			"pinning) on the direct sibling / router project yamls of this "+
			"run before spawn. One-hop only — the flag does not propagate "+
			"to child invocations, so a sibling's own siblings get default "+
			"gates (no H3 escalation). Off by default; opt-in for CI / "+
			"paranoid setups (ADR-036 § Optional escape hatch).")
	upCmd.Flags().BoolVar(&routerOff, "router-off", false,
		"Bypass the workspace router project (ADR-037) and run the bundled "+
			"Caddy as before v0.8. In meta mode, prevents the meta runner "+
			"from setting RAIOZ_ROUTER_ACTIVE=1 on consumers; in project "+
			"mode, overrides an inherited RAIOZ_ROUTER_ACTIVE=1 so the "+
			"bundled Caddy still starts. No-op when no router is in play.")
	upCmd.Flags().BoolVar(&noClone, "no-clone", false,
		"Skip the meta auto-clone bootstrap (ADR-048). Sub-projects "+
			"declared with `git:` whose path is missing on disk will fail "+
			"(or skip if optional) instead of being cloned. Useful for "+
			"offline reproductions. No-op outside meta mode.")
	upCmd.Flags().StringSliceVar(&forceRemote, "force-remote", nil,
		"Force the named meta sub-projects to MetaModeRemote (ADR-049), "+
			"regardless of filesystem presence. Each name must match a "+
			"projects[].path basename AND have a `remote:` URL declared. "+
			"Comma-separated or repeatable. No-op outside meta mode.")
}
