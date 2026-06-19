package app

import (
	"context"
	"os/exec"
	"sort"

	"raioz/internal/i18n"
	"raioz/internal/logging"
	"raioz/internal/naming"
	"raioz/internal/refcount"
	"raioz/internal/runtime"
)

// composeDownProject tears an orphaned dep's compose project down. Indirected
// so tests can exercise the GC without a Docker daemon. The liveness probe
// goes through the injected DockerRunner port (ContainerManager.IsProjectActive),
// so it needs no indirection here.
var composeDownProject = func(ctx context.Context, projName string) ([]byte, error) {
	args := []string{"compose", "-p", projName, "down", "--remove-orphans"}
	return exec.CommandContext(ctx, runtime.Binary(), args...).CombinedOutput()
}

// refGCScope returns the workspaces whose stale refs clean should sweep:
// every tracked workspace for `--all`, otherwise just the current project's
// workspace. A refcount-read failure under `--all` degrades to no-op.
func (uc *CleanUseCase) refGCScope(all bool, workspaceName string) []string {
	if !all {
		return []string{workspaceName}
	}
	workspaces, err := refcount.Workspaces()
	if err != nil {
		logging.Warn("Could not enumerate workspaces for shared-dep GC", "error", err.Error())
		return nil
	}
	return workspaces
}

// pruneStaleSharedRefs is the clean-time garbage collector for the shared
// dependency refcount (ADR-050). A reference left behind by a project that
// is no longer running pins a shared dep up forever, because `down` only
// ever drops the LEAVING project's own ref; without this GC the only remedy
// is hand-editing shared-deps.json.
//
// For every workspace in scope it walks the refcount and, for each
// referencing project, probes whether that project currently has a running
// raioz-managed container (ContainerManager.IsProjectActive — running-only,
// no fail-open). A ref whose project is NOT running is dropped; a dep left
// with zero live references is torn down.
//
// Why this lives in `clean` and not in the `down` hot path: the liveness
// probe is a container scan, and a consumer that uses ONLY shared deps owns
// no project-labeled container (ADR-002), so the probe reads it as gone.
// ADR-050 deliberately refused to act on that signal during `down` — a
// smoke test caught it tearing a dep out from under a live shared-deps-only
// sibling. `clean` is explicit, recoverable maintenance (a wrongly-pruned
// ref is re-created by the next `up`), so the trade-off is acceptable here
// where it is not on the down path.
//
// Conservative on uncertainty: if the probe ERRORS (daemon unreachable,
// timeout) the project is treated as live and its ref is kept — never prune
// on a probe we could not complete.
func (uc *CleanUseCase) pruneStaleSharedRefs(
	ctx context.Context, workspaces []string, dryRun bool,
) []string {
	var actions []string
	for _, ws := range workspaces {
		actions = append(actions, uc.pruneWorkspaceRefs(ctx, ws, dryRun)...)
	}
	return actions
}

// pruneWorkspaceRefs GCs stale refs for a single workspace. The naming
// prefix is set per workspace so SharedDepComposeProjectName resolves the
// same scope ImageRunner used at up time.
func (uc *CleanUseCase) pruneWorkspaceRefs(
	ctx context.Context, ws string, dryRun bool,
) []string {
	snapshot, err := refcount.Snapshot(ws)
	if err != nil {
		logging.WarnWithContext(ctx, "Could not read shared-dep refcount for clean",
			"workspace", ws, "error", err.Error())
		return nil
	}
	if len(snapshot) == 0 {
		return nil
	}
	naming.SetPrefix(ws)

	deps := make([]string, 0, len(snapshot))
	for dep := range snapshot {
		deps = append(deps, dep)
	}
	sort.Strings(deps)

	var actions []string
	for _, dep := range deps {
		live, dead := uc.classifyRefs(ctx, ws, dep, snapshot[dep])
		if len(dead) == 0 {
			continue // nothing stale for this dep
		}
		for _, p := range dead {
			actions = append(actions, i18n.T("clean.pruned_stale_ref", dep, p, ws))
		}
		orphaned := len(live) == 0

		if dryRun {
			if orphaned {
				actions = append(actions, i18n.T("clean.would_teardown_orphan_dep",
					dep, naming.SharedDepComposeProjectName(dep)))
			}
			continue
		}

		uc.dropRefs(ctx, ws, dep, dead)
		if orphaned {
			actions = append(actions, uc.teardownOrphanDep(ctx, dep))
		}
	}
	return actions
}

// classifyRefs probes each referencing project's liveness and splits the
// refs into live (running, or probe-failed → kept on uncertainty) and dead
// (confirmed not running). It performs no writes.
func (uc *CleanUseCase) classifyRefs(
	ctx context.Context, ws, dep string, refs []string,
) (live, dead []string) {
	for _, project := range refs {
		active, err := uc.deps.DockerRunner.IsProjectActive(ctx, ws, project)
		if err != nil {
			logging.WarnWithContext(ctx, "Liveness probe failed; keeping shared-dep ref",
				"workspace", ws, "dep", dep, "project", project, "error", err.Error())
			live = append(live, project)
			continue
		}
		if active {
			live = append(live, project)
			continue
		}
		dead = append(dead, project)
	}
	return live, dead
}

// dropRefs removes each dead project's reference to dep from the refcount.
func (uc *CleanUseCase) dropRefs(ctx context.Context, ws, dep string, dead []string) {
	for _, project := range dead {
		if _, err := refcount.DropRef(ws, dep, project); err != nil {
			logging.WarnWithContext(ctx, "Failed to drop stale ref",
				"workspace", ws, "dep", dep, "project", project, "error", err.Error())
		}
	}
}

// teardownOrphanDep tears the dep's workspace-shared compose project down.
// Called only after every referencing project was confirmed not running, so
// no live consumer can be using it. Returns an action string.
func (uc *CleanUseCase) teardownOrphanDep(ctx context.Context, dep string) string {
	projName := naming.SharedDepComposeProjectName(dep)
	if out, err := composeDownProject(ctx, projName); err != nil {
		logging.WarnWithContext(ctx, "Orphan shared-dep teardown failed",
			"dep", dep, "project", projName, "error", err.Error(), "output", string(out))
		return i18n.T("clean.orphan_dep_teardown_failed", dep)
	}
	return i18n.T("clean.tore_down_orphan_dep", dep, projName)
}
