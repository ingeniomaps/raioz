package cli

import (
	"context"
	"fmt"

	"raioz/internal/app"
	"raioz/internal/config"
	"raioz/internal/i18n"
	infraproxy "raioz/internal/infra/proxy"
	"raioz/internal/output"
)

// newMetaRunner is the constructor tryHandleMeta uses to obtain a
// MetaRunner. Tests override it to inject a fake binary (Binary field)
// without monkey-patching os.Args[0] — the production MetaRunner
// resolves its binary via os.Executable() (ADR-008), which under
// `go test` returns the test runner and recursively re-enters the
// suite. Package-level var to keep the override surface tiny.
var newMetaRunner = func() *app.MetaRunner { return &app.MetaRunner{} }

// defaultRemoteRouteWriter is the production binding the meta bootstrap
// uses to publish workspace Caddy routes for remote-mode sub-projects.
// Lives in the cli layer (not in app/) so app/ stays free of an
// internal/proxy import per ADR-029.
var defaultRemoteRouteWriter = app.RemoteRouteWriter(infraproxy.WriteRemoteRoutes)

// tryHandleMeta detects whether configPath points at a meta-orchestrator
// raioz.yaml (`kind: meta`). If yes, it dispatches to MetaRunner with the
// matching sub-command and returns (handled=true, err). If the config is a
// regular project, it returns (false, nil) so the caller proceeds with the
// normal use case. activeProfiles filters which sub-projects participate
// in `up` / `status`; `down` ignores it (see MetaRunner.Down). opts is
// shared with app.MetaUpOptions so a new knob lands in one place; the
// fields are ignored for non-up subcommands.
func tryHandleMeta(
	ctx context.Context, configPath, subCmd string,
	args, activeProfiles []string, opts app.MetaUpOptions,
) (bool, error) {
	if configPath == "" || configPath == AutoDetectMarker {
		return false, nil
	}

	cfg, isMeta, err := config.LoadMetaConfig(configPath)
	if err != nil {
		// Surface meta-config errors immediately rather than letting the
		// normal loader try to parse the same file as a project config —
		// the second pass would emit a confusing "missing project name" or
		// similar.
		if isMeta {
			return true, fmt.Errorf("meta config %q: %w", configPath, err)
		}
		// Not a meta config and not a parse error we own — fall through.
		return false, nil
	}
	if !isMeta {
		return false, nil
	}

	output.PrintInfo(i18n.T("meta.dispatch.header", len(cfg.Projects)))

	runner := newMetaRunner()
	if opts.RemoteRouteWriter == nil {
		opts.RemoteRouteWriter = defaultRemoteRouteWriter
	}
	var summary app.MetaSummaryList
	switch subCmd {
	case "up":
		summary = runner.Up(ctx, cfg, args, activeProfiles, opts)
	case "down":
		summary = runner.Down(ctx, cfg, args)
	case "status":
		summary = runner.Status(ctx, cfg, args, activeProfiles)
	default:
		return false, fmt.Errorf("meta dispatch: unsupported subcommand %q", subCmd)
	}

	printMetaSummary(subCmd, summary)
	if summary.HasFailures() {
		return true, fmt.Errorf("meta %s: one or more projects failed", subCmd)
	}
	return true, nil
}

func printMetaSummary(subCmd string, summary app.MetaSummaryList) {
	output.PrintInfo("")
	output.PrintInfo(i18n.T("meta.summary.header", subCmd))
	for _, e := range summary {
		switch {
		case e.Err == nil:
			output.PrintSuccess(i18n.T("meta.summary.row_ok", e.Project))
		case e.Skipped:
			output.PrintWarning(i18n.T("meta.summary.row_skipped", e.Project, e.Err))
		default:
			output.PrintError(i18n.T("meta.summary.row_failed", e.Project, e.Err))
		}
	}
}
