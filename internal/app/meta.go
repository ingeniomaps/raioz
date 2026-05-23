package app

import (
	"context"
	"fmt"
	"os"
	"time"

	"raioz/internal/audit"
	"raioz/internal/config"
	"raioz/internal/i18n"
	"raioz/internal/output"
	"raioz/internal/protocol"
)

// MetaRunner orchestrates a meta-orchestrator config: a raioz.yaml with
// `kind: meta` whose sole job is to delegate up/down/status to N sub-projects
// in order. Each sub-project keeps its own raioz.yaml, .raioz.state.json, and
// lifecycle.
//
// The implementation deliberately shells out to the current binary
// (os.Args[0]) instead of re-using the in-process use cases. Two reasons:
//   - Each sub-project gets a clean process with its own config loader,
//     i18n state, and naming prefix. No global-state contamination.
//   - Failure isolation is automatic: a panic / fatal in one sub doesn't
//     drag the meta runner down.
type MetaRunner struct {
	// Binary is the raioz executable to invoke for sub-projects. Defaults
	// to os.Args[0] when empty. Tests inject a fake here.
	Binary string
	// Stdout / Stderr default to os.Stdout / os.Stderr. Tests inject buffers.
	Stdout, Stderr *os.File
}

// MetaSummary describes the outcome of a meta run, one entry per sub-project.
type MetaSummary struct {
	Project string
	Path    string
	Err     error // nil on success
	Skipped bool  // true when an optional sub failed and was tolerated
}

// MetaSummaryList is a typed slice so HasFailures can hang off a method.
// Defining it lets callers write `summary.HasFailures()` instead of needing
// a free function.
type MetaSummaryList []MetaSummary

// HasFailures reports whether any non-optional sub failed. Used as the
// command-level exit status.
func (s MetaSummaryList) HasFailures() bool {
	for _, e := range s {
		if e.Err != nil && !e.Skipped {
			return true
		}
	}
	return false
}

// MetaUpOptions tunes a meta up run. RouterOff bypasses the ADR-037
// router phase for debugging. AuditSiblings opts into the ADR-036
// preflight (H1/H2/H3 gates against every router / sub-project yaml).
// NoClone bypasses the ADR-048 auto-clone bootstrap (CLI `--no-clone`).
// Cloner overrides the bootstrap's clone primitive — tests inject a
// fake here so the unit tests don't shell out to git.
// ForceRemote lists project names to bump to MetaModeRemote regardless
// of filesystem presence — drives ADR-049's `--force-remote=` flag.
// RemoteRouteWriter overrides the proxy route-file writer (tests
// inject a fake to avoid writing to the user's XDG state dir).
type MetaUpOptions struct {
	RouterOff         bool
	AuditSiblings     bool
	NoClone           bool
	Cloner            MetaCloner
	ForceRemote       []string
	RemoteRouteWriter RemoteRouteWriter
}

// Up runs `raioz up` in each sub-project, in order. Optional subs
// that fail are reported but don't abort. activeProfiles filters to
// always-on projects + ones with a matching profile. When cfg.Router
// is set and opts.RouterOff is false, the router project comes up
// first; consumers then run with RAIOZ_ROUTER_ACTIVE=1.
func (m *MetaRunner) Up(
	ctx context.Context, cfg *config.MetaConfig,
	args, activeProfiles []string, opts MetaUpOptions,
) (results MetaSummaryList) {
	start := time.Now()
	_ = audit.LogLifecycleStart(ctx, "meta_up", metaAuditTarget(cfg), cfg.Workspace)
	defer func() {
		logMetaLifecycleComplete(ctx, "meta_up", cfg, results, start)
	}()

	// Opt-in preflight (ADR-036): scan every router + sub-project
	// yaml for H1/H2/H3 violations before any spawn. Failure aborts
	// the whole meta up so no sibling gets a chance to run with a
	// surface raioz wouldn't have accepted itself.
	if opts.AuditSiblings {
		if err := auditMetaTargets(cfg); err != nil {
			results = append(results, MetaSummary{
				Project: "audit-siblings", Err: err,
			})
			return results
		}
	}

	// Force-remote runs BEFORE bootstrap so `--force-remote=api`
	// short-circuits the clone attempt even when the path exists
	// locally. See ADR-049.
	if err := applyForceRemote(cfg, opts.ForceRemote); err != nil {
		results = append(results, MetaSummary{Project: "force-remote", Err: err})
		return results
	}

	// --no-clone bypasses the clone half only — remote-mode writes
	// still happen because they're not network I/O against an unknown
	// repo. See ADR-048 + ADR-049 for the cascade contract.
	if !opts.NoClone {
		bootstrapResults, err := m.bootstrapMeta(ctx, cfg, opts.Cloner, opts.RemoteRouteWriter)
		results = append(results, bootstrapResults...)
		if err != nil {
			return results
		}
	}

	var consumerEnv []string
	var initialCompleted []string
	skipPath := ""

	if cfg.Router != nil && !opts.RouterOff {
		// Phase 1: router first. The router project is treated as a
		// non-optional sub — a failure aborts the whole meta up. No
		// RAIOZ_ROUTER_ACTIVE here: the router project itself owns the
		// edge routing (it IS the proxy), so its own Caddy/whatever
		// must come up normally.
		//
		// Pass RAIOZ_ROUTER_ASSIGNED_IP so the router can
		// bind the conventional bundled-Caddy IP and the operator's
		// /etc/hosts / proxy.publish:false setup keeps working when
		// swapping between bundled Caddy and the router project.
		routerEnv := routerHandoffEnv(cfg)
		entry := m.runSingle(ctx, "up", *cfg.Router, args, routerEnv)
		results = append(results, entry)
		if entry.Err != nil {
			return results
		}
		consumerEnv = []string{protocol.RouterActive + "=1"}
		if cfg.Router.Name != "" {
			// Seed the meta-completed list so consumers that declare
			// the router as a sibling dep skip the redundant spawn.
			initialCompleted = append(initialCompleted, cfg.Router.Name)
		}
		skipPath = cfg.Router.Path
	}

	consumers := m.run(
		ctx, cfg, "up", args, activeProfiles, false,
		consumerEnv, skipPath, initialCompleted,
	)
	results = append(results, consumers...)
	return results
}

// Down runs `raioz down` in each sub-project in REVERSE order. Errors are
// always tolerated — teardown should be best-effort. Profiles are
// deliberately ignored here so a sub-project started under a different
// `--meta-profile` set still gets cleaned up; you can't strand a service
// you brought up earlier. When cfg.Router is non-nil the router goes
// down LAST — every consumer must be torn down before its edge dies.
func (m *MetaRunner) Down(
	ctx context.Context, cfg *config.MetaConfig, args []string,
) (results MetaSummaryList) {
	start := time.Now()
	_ = audit.LogLifecycleStart(ctx, "meta_down", metaAuditTarget(cfg), cfg.Workspace)
	defer func() {
		logMetaLifecycleComplete(ctx, "meta_down", cfg, results, start)
	}()

	skipPath := ""
	if cfg.Router != nil {
		skipPath = cfg.Router.Path
	}
	results = m.run(ctx, cfg, "down", args, nil, true, nil, skipPath, nil)
	if cfg.Router != nil {
		results = append(results, m.runSingle(ctx, "down", *cfg.Router, args, nil))
	}
	return results
}

// Status runs `raioz status` in each sub-project, in order. Errors are
// tolerated so a single missing sub doesn't blank the rest of the report.
// Respects activeProfiles for symmetry with Up — the report shows what
// the matching `raioz up` would have started. When cfg.Router is set the
// router is reported first, then consumers in declared order.
func (m *MetaRunner) Status(
	ctx context.Context, cfg *config.MetaConfig,
	args, activeProfiles []string,
) (results MetaSummaryList) {
	start := time.Now()
	_ = audit.LogLifecycleStart(ctx, "meta_status", metaAuditTarget(cfg), cfg.Workspace)
	defer func() {
		logMetaLifecycleComplete(ctx, "meta_status", cfg, results, start)
	}()

	skipPath := ""
	if cfg.Router != nil {
		results = append(results, m.runSingle(ctx, "status", *cfg.Router, args, nil))
		skipPath = cfg.Router.Path
	}
	rest := m.run(ctx, cfg, "status", args, activeProfiles, false, nil, skipPath, nil)
	results = append(results, rest...)
	return results
}

// shouldIncludeMetaProject decides whether a project participates in this
// meta run given the active profile set. Empty Profiles = always-on
// regardless of the active list. Non-empty Profiles require at least one
// match. A nil/empty activeProfiles list keeps only the always-on
// projects.
func shouldIncludeMetaProject(p config.MetaProject, active []string) bool {
	if len(p.Profiles) == 0 {
		return true
	}
	for _, a := range active {
		for _, pp := range p.Profiles {
			if pp == a {
				return true
			}
		}
	}
	return false
}

func (m *MetaRunner) run(
	ctx context.Context, cfg *config.MetaConfig,
	subCmd string, extraArgs, activeProfiles []string, reverse bool,
	extraEnv []string, skipPath string, initialCompleted []string,
) MetaSummaryList {
	projects := cfg.Projects
	if len(activeProfiles) > 0 || !reverse {
		// Filter by profile for up/status. Down passes activeProfiles=nil
		// so this branch keeps the full list.
		filtered := projects[:0:0]
		for _, p := range projects {
			if shouldIncludeMetaProject(p, activeProfiles) {
				filtered = append(filtered, p)
			}
		}
		projects = filtered
	}
	if reverse {
		projects = reverseMetaProjects(projects)
	}

	results := make(MetaSummaryList, 0, len(projects))
	// completed grows as each `up` sub returns ok; stamped into the
	// env of subsequent sub-ups (issue 020-meta).
	completed := append([]string(nil), initialCompleted...)
	for _, p := range projects {
		if skipPath != "" && p.Path == skipPath {
			continue
		}
		// Skip = bootstrap dropped this project (ADR-048).
		// Remote = bootstrap published a Caddy route for it (ADR-049);
		// the local Caddy serves it, no sub-process to spawn.
		if p.Mode == config.MetaModeSkip || p.Mode == config.MetaModeRemote {
			continue
		}
		subEnv := withMetaCompleted(extraEnv, completed, subCmd)
		entry := m.runSingle(ctx, subCmd, p, extraArgs, subEnv)
		switch {
		case entry.Err == nil:
			if subCmd == "up" && p.Name != "" {
				completed = append(completed, p.Name)
			}
		case p.Optional && subCmd == "up":
			entry.Skipped = true
			output.PrintWarning(
				i18n.T("meta.optional_failed", p.Name, entry.Err),
			)
			_ = audit.LogWithContext(
				ctx,
				audit.EventTypeLifecycle,
				metaSubFailureDetails(subCmd, p, entry.Err, true),
				fmt.Sprintf("meta_sub_%s skipped: %s", subCmd, p.Name),
			)
		case subCmd == "down" || subCmd == "status":
			// Best-effort: keep going on remaining subs even if this one
			// errored. The error is recorded in the summary.
			output.PrintWarning(
				i18n.T("meta.sub_error_continuing", subCmd, p.Name, entry.Err),
			)
			_ = audit.LogWithContext(
				ctx,
				audit.EventTypeLifecycle,
				metaSubFailureDetails(subCmd, p, entry.Err, false),
				fmt.Sprintf("meta_sub_%s failed: %s", subCmd, p.Name),
			)
		default:
			results = append(results, entry)
			return results // hard fail on first non-optional up failure
		}
		results = append(results, entry)
	}
	return results
}

func reverseMetaProjects(in []config.MetaProject) []config.MetaProject {
	out := make([]config.MetaProject, len(in))
	for i, p := range in {
		out[len(in)-1-i] = p
	}
	return out
}
