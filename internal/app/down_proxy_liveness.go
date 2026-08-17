// Liveness probes behind `raioz down`'s destructive decisions: pruning a
// sibling's route file and stopping the shared proxy. Every probe here fails
// closed — ADR-005 requires positive proof of death.

package app

import (
	"context"
	"fmt"

	"raioz/internal/docker"
	"raioz/internal/host"
	"raioz/internal/logging"
	"raioz/internal/naming"
	"raioz/internal/state"
)

// Hooks so tests can simulate any mix of sibling liveness without a daemon.
// The Err variant is separate on purpose: callers that delete state must tell
// "docker unreachable" apart from "no containers".
var (
	listContainersByLabelsFn    = docker.ListContainersByLabels
	getContainerLabelFn         = docker.GetContainerLabel
	listContainersByLabelsErrFn = docker.ListContainersByLabelsErr
	hostPIDAliveFn              = host.IsProcessAlive
	anyContainerRunningFn       = docker.AnyContainerRunning
)

// pruneOrphanRouteFiles removes route files whose owning project shows no
// sign of life. A project that crashed without running `raioz down` otherwise
// leaves an immortal file that pins the shared proxy and injects a dead
// backend into every Caddyfile reload (ADR-005).
//
// A failed docker probe proves nothing, so the whole GC is skipped.
func (uc *DownUseCase) pruneOrphanRouteFiles(ctx context.Context, workspace, currentProject string) {
	if workspace == "" {
		return
	}
	withRoutes := uc.deps.ProxyManager.ListProjectsWithRoutes()
	if len(withRoutes) == 0 {
		return
	}

	live, err := liveWorkspaceProjects(ctx, workspace)
	if err != nil {
		logging.WarnWithContext(ctx, "Skipping orphan route GC: docker liveness probe failed",
			"workspace", workspace, "error", err.Error())
		return
	}

	for _, proj := range withRoutes {
		if proj == currentProject {
			continue // our own file is handled by RemoveProjectRoutes
		}
		if _, alive := live[proj]; alive {
			continue
		}
		if uc.routeOwnerAliveOnHost(ctx, workspace, proj) {
			continue
		}
		// Launcher-pattern projects daemonize: the recorded PID is the
		// launcher's and their containers carry no raioz labels, so both
		// probes above read them as dead. Their route targets don't.
		if uc.routeTargetAliveInDocker(ctx, workspace, proj) {
			continue
		}
		if err := uc.deps.ProxyManager.RemoveRoutesFor(proj); err != nil {
			logging.WarnWithContext(ctx, "Failed to prune orphan route file",
				"workspace", workspace, "project", proj, "error", err.Error())
			continue
		}
		logging.InfoWithContext(ctx, "Pruned orphan route file (project has no live containers or host processes)",
			"workspace", workspace, "project", proj)
	}
}

// routeOwnerAliveOnHost reports whether the project owning a route file has a
// live host PID, the only signal left by services running via `command:`.
//
// Returns true whenever liveness cannot be determined — a route file without
// a projectDir (legacy or remote, ADR-049), or an unreadable state file —
// because ADR-005 prunes only on positive proof of death.
func (uc *DownUseCase) routeOwnerAliveOnHost(ctx context.Context, workspace, project string) bool {
	dir := uc.deps.ProxyManager.ProjectDirFor(project)
	if dir == "" {
		logging.DebugWithContext(ctx, "Keeping route file: owner liveness unknown (no projectDir persisted)",
			"workspace", workspace, "project", project)
		return true
	}
	st, err := state.LoadLocalState(dir)
	if err != nil {
		logging.DebugWithContext(ctx, "Keeping route file: owner state unreadable",
			"workspace", workspace, "project", project, "dir", dir, "error", err.Error())
		return true
	}
	for name, pid := range st.HostPIDs {
		if pid > 0 && hostPIDAliveFn(pid) {
			logging.InfoWithContext(ctx, "Keeping route file: owner has a live host process",
				"workspace", workspace, "project", project, "service", name, "pid", pid)
			return true
		}
	}
	return false
}

// routeTargetAliveInDocker reports whether any container the project's route
// file targets is running — the only liveness signal a launcher-pattern
// project leaves behind. No container targets means no signal (false); a
// probe error keeps the file (true), per ADR-005.
func (uc *DownUseCase) routeTargetAliveInDocker(ctx context.Context, workspace, project string) bool {
	targets := uc.deps.ProxyManager.RouteTargetsFor(project)
	if len(targets) == 0 {
		return false
	}
	running, err := anyContainerRunningFn(ctx, targets)
	if err != nil {
		logging.WarnWithContext(ctx, "Keeping route file: target liveness probe failed",
			"workspace", workspace, "project", project, "error", err.Error())
		return true
	}
	if running {
		logging.InfoWithContext(ctx, "Keeping route file: owner has a live route-target container",
			"workspace", workspace, "project", project)
	}
	return running
}

// liveWorkspaceProjects returns the set of project names that currently have
// at least one raioz-managed container alive in the workspace. Returns an
// error if Docker cannot be reached — callers that delete state on the basis
// of absence MUST treat that as "unknown", not "empty".
func liveWorkspaceProjects(ctx context.Context, workspace string) (map[string]struct{}, error) {
	names, err := listContainersByLabelsErrFn(ctx, map[string]string{
		naming.LabelManaged:   "true",
		naming.LabelWorkspace: workspace,
	})
	if err != nil {
		return nil, err
	}
	live := make(map[string]struct{}, len(names))
	for _, n := range names {
		proj, err := getContainerLabelFn(ctx, n, naming.LabelProject)
		if err != nil {
			return nil, fmt.Errorf("inspect container %s: %w", n, err)
		}
		if proj == "" {
			continue // shared dep or the proxy itself — not a project consumer
		}
		live[proj] = struct{}{}
	}
	return live, nil
}

// otherWorkspaceProjectsActive reports whether any raioz-managed container
// in the workspace belongs to a project other than the one currently being
// torn down. Used to decide whether the shared proxy can be stopped.
//
// A live shared dep (empty project label, ADR-002) vetoes the teardown: deps
// outlive individual downs until the last consumer leaves, so one still
// running means some consumer remains — possibly a `command:` project that
// owns no labeled container at all. The leaving project's own deps are
// already stopped by the time this runs, so they can't false-positive.
func otherWorkspaceProjectsActive(ctx context.Context, workspace, currentProject string) bool {
	if workspace == "" {
		return false
	}
	names := listContainersByLabelsFn(ctx, map[string]string{
		naming.LabelManaged:   "true",
		naming.LabelWorkspace: workspace,
	})
	for _, n := range names {
		proj, _ := getContainerLabelFn(ctx, n, naming.LabelProject)
		if proj != "" {
			if proj != currentProject {
				return true
			}
			continue
		}
		kind, _ := getContainerLabelFn(ctx, n, naming.LabelKind)
		if kind == naming.KindDependency {
			return true // live shared dep ⇒ some consumer remains (ADR-002)
		}
		// kind proxy (or unknown): still not evidence of a consumer.
	}
	return false
}

// workspaceSiblingAlive reports whether any OTHER project in the workspace is
// still up, by labels or — for launcher-pattern projects, which have none — by
// route targets. The second probe only matters under --all/--prune-shared:
// without force a surviving route file keeps the proxy alive anyway.
func (uc *DownUseCase) workspaceSiblingAlive(ctx context.Context, workspace, currentProject string) bool {
	if otherWorkspaceProjectsActive(ctx, workspace, currentProject) {
		return true
	}
	for _, proj := range uc.deps.ProxyManager.ListProjectsWithRoutes() {
		if proj == currentProject {
			continue
		}
		if uc.routeTargetAliveInDocker(ctx, workspace, proj) {
			return true
		}
	}
	return false
}
