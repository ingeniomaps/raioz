package app

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"raioz/internal/audit"
	"raioz/internal/config"
	"raioz/internal/host"
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

// MetaUpOptions tunes a meta up run. RouterOff bypasses the ADR-037 router
// phase even when the config declares `router:`, restoring pre-v0.8
// behavior for debugging. Future per-run knobs land here without
// breaking callers.
type MetaUpOptions struct {
	RouterOff bool
	// AuditSiblings enables the opt-in preflight that runs ADR-036
	// hygiene gates against every router / sub-project yaml before
	// spawn. Off by default (transitive trust is the documented v0.7+
	// policy). See ADR-040 § Optional escape hatch and issue 031.
	AuditSiblings bool
}

// Up runs `raioz up` in each sub-project, in order. Optional subs that fail
// are reported but don't abort the meta run. activeProfiles narrows the
// iteration to projects that have no Profiles declared (always-on) or
// whose Profiles intersect the list. When cfg.Router is non-nil and
// opts.RouterOff is false, the router project comes up first; consumers
// then run with RAIOZ_ROUTER_ACTIVE=1 so they skip their bundled Caddy.
func (m *MetaRunner) Up(
	ctx context.Context, cfg *config.MetaConfig,
	args, activeProfiles []string, opts MetaUpOptions,
) (results MetaSummaryList) {
	start := time.Now()
	_ = audit.LogLifecycleStart(ctx, "meta_up", metaAuditTarget(cfg), cfg.Workspace)
	defer func() {
		logMetaLifecycleComplete(ctx, "meta_up", cfg, results, start)
	}()

	// Opt-in preflight (issue 031): scan every router + sub-project
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

	var consumerEnv []string
	skipPath := ""

	if cfg.Router != nil && !opts.RouterOff {
		// Phase 1: router first. The router project is treated as a
		// non-optional sub — a failure aborts the whole meta up. No
		// RAIOZ_ROUTER_ACTIVE here: the router project itself owns the
		// edge routing (it IS the proxy), so its own Caddy/whatever
		// must come up normally.
		entry := m.runSingle(ctx, "up", *cfg.Router, args, nil)
		results = append(results, entry)
		if entry.Err != nil {
			return results
		}
		consumerEnv = []string{protocol.RouterActive + "=1"}
		skipPath = cfg.Router.Path
	}

	consumers := m.run(
		ctx, cfg, "up", args, activeProfiles, false, consumerEnv, skipPath,
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
	results = m.run(ctx, cfg, "down", args, nil, true, nil, skipPath)
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
	rest := m.run(ctx, cfg, "status", args, activeProfiles, false, nil, skipPath)
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
	extraEnv []string, skipPath string,
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
	for _, p := range projects {
		if skipPath != "" && p.Path == skipPath {
			// Router project already handled in the dedicated phase
			// (Up phase 1 / Down phase 2). Skipping here prevents a
			// double-up that would race with the dedicated phase.
			continue
		}
		entry := m.runSingle(ctx, subCmd, p, extraArgs, extraEnv)
		switch {
		case entry.Err == nil:
			// success
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

// runSingle invokes the raioz binary once for a single sub-project and
// returns the per-project summary entry. extraEnv is appended to the
// inherited process environment so callers can layer flags (router
// active, future per-call signals) without rewriting the whole env.
func (m *MetaRunner) runSingle(
	ctx context.Context, subCmd string,
	p config.MetaProject, extraArgs, extraEnv []string,
) MetaSummary {
	binary, err := m.resolveBinary()
	if err != nil {
		return MetaSummary{Project: p.Name, Path: p.Path, Err: err}
	}
	stdout := m.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}
	stderr := m.Stderr
	if stderr == nil {
		stderr = os.Stderr
	}

	printMetaBanner(stdout, subCmd, p)

	cmd := m.buildSubCmd(ctx, binary, subCmd, p, extraArgs, extraEnv, stdout, stderr)
	runErr := cmd.Run()
	return MetaSummary{Project: p.Name, Path: p.Path, Err: runErr}
}

// resolveBinary picks the raioz executable to invoke for a sub-project
// spawn. Resolution order:
//
//  1. m.Binary when set (tests inject a fake binary here).
//  2. os.Executable() — the path the kernel sees for this process. Stable
//     under PATH changes and survives cwd switches.
//  3. filepath.Abs(os.Args[0]) as a last-resort fallback. Required because
//     runSingle sets cmd.Dir to the sub-project path before exec, which
//     turns a relative os.Args[0] (e.g. "./raioz" from a dev build) into
//     an unfindable path inside the sub-project dir.
func (m *MetaRunner) resolveBinary() (string, error) {
	if m.Binary != "" {
		return m.Binary, nil
	}
	if exe, err := os.Executable(); err == nil && exe != "" {
		return exe, nil
	}
	if len(os.Args) > 0 && os.Args[0] != "" {
		abs, err := filepath.Abs(os.Args[0])
		if err != nil {
			return "", fmt.Errorf("resolve raioz binary path: %w", err)
		}
		return abs, nil
	}
	return "", fmt.Errorf("cannot resolve raioz binary path for meta dispatch")
}

// buildSubCmd constructs the *exec.Cmd for a sub-project invocation.
// Split out from runSingle so tests can inspect SysProcAttr without
// running the binary.
func (m *MetaRunner) buildSubCmd(
	ctx context.Context, binary, subCmd string, p config.MetaProject,
	extraArgs, extraEnv []string, stdout, stderr *os.File,
) *exec.Cmd {
	args := append([]string{subCmd}, extraArgs...)
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = p.Path
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	env := os.Environ()
	if len(extraEnv) > 0 {
		env = append(env, extraEnv...)
	}
	cmd.Env = env
	// Router + consumer subprocesses must die with the meta parent;
	// otherwise a SIGKILL leaves N raioz children each mid-`docker
	// compose up`, each still holding their own project locks.
	host.AttachPdeathsig(cmd)
	return cmd
}

func reverseMetaProjects(in []config.MetaProject) []config.MetaProject {
	out := make([]config.MetaProject, len(in))
	for i, p := range in {
		out[len(in)-1-i] = p
	}
	return out
}

func printMetaBanner(w *os.File, subCmd string, p config.MetaProject) {
	tag := strings.ToUpper(subCmd)
	if p.Optional {
		fmt.Fprintln(w, "\n"+i18n.T("meta.banner_optional", tag, p.Name))
	} else {
		fmt.Fprintln(w, "\n"+i18n.T("meta.banner", tag, p.Name))
	}
}
