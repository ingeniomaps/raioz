package cli

import (
	"context"
	"fmt"

	"raioz/internal/app"
	"raioz/internal/config"
	"raioz/internal/output"
)

// tryHandleMeta detects whether configPath points at a meta-orchestrator
// raioz.yaml (`kind: meta`). If yes, it dispatches to MetaRunner with the
// matching sub-command and returns (handled=true, err). If the config is a
// regular project, it returns (false, nil) so the caller proceeds with the
// normal use case.
func tryHandleMeta(
	ctx context.Context, configPath, subCmd string, args []string,
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

	output.PrintInfo(fmt.Sprintf(
		"meta-orchestrator: %d project(s)", len(cfg.Projects),
	))

	runner := &app.MetaRunner{}
	var summary app.MetaSummaryList
	switch subCmd {
	case "up":
		summary = runner.Up(ctx, cfg, args)
	case "down":
		summary = runner.Down(ctx, cfg, args)
	case "status":
		summary = runner.Status(ctx, cfg, args)
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
	output.PrintInfo(fmt.Sprintf("=== meta %s summary ===", subCmd))
	for _, e := range summary {
		switch {
		case e.Err == nil:
			output.PrintSuccess(fmt.Sprintf("  ok   %s", e.Project))
		case e.Skipped:
			output.PrintWarning(fmt.Sprintf("  skip %s (%s)", e.Project, e.Err))
		default:
			output.PrintError(fmt.Sprintf("  fail %s (%s)", e.Project, e.Err))
		}
	}
}
