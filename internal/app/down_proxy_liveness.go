// Liveness probes behind `raioz down`'s two destructive decisions: pruning a
// sibling's persisted route file and stopping the workspace-shared proxy.
// Both follow ADR-005's rule — positive proof of death, never mere absence of
// proof of life — so every probe here fails closed. Split out of
// down_proxy.go, which owns the lifecycle itself.

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

// listContainersByLabelsFn / getContainerLabelFn are package-level hooks for
// the workspace-occupancy probe. Production points at docker.* directly;
// tests stub them so they can simulate any mix of sibling presence without
// a real Docker daemon.
var listContainersByLabelsFn = docker.ListContainersByLabels
var getContainerLabelFn = docker.GetContainerLabel

// listContainersByLabelsErrFn is the error-surfacing probe used by the
// orphan-route GC. It is a separate hook from listContainersByLabelsFn
// because the GC makes a destructive decision (deleting route files) and
// MUST be able to tell "docker unreachable" apart from "no containers".
var listContainersByLabelsErrFn = docker.ListContainersByLabelsErr

// hostPIDAliveFn is the host-process liveness probe used by the orphan-route
// GC. Package-level hook for the same reason as the container probes above:
// the GC makes a destructive decision and tests must be able to simulate a
// live/dead sibling host process without spawning one.
var hostPIDAliveFn = host.IsProcessAlive

// anyContainerRunningFn is the route-target liveness probe used by the
// orphan-route GC. Hookable like the probes above so tests can simulate a
// live/dead launcher-pattern backend without a docker daemon.
var anyContainerRunningFn = docker.AnyContainerRunning

// pruneOrphanRouteFiles removes persisted route files whose owning project
// shows no sign of life — neither a labeled container in the workspace nor
// a live host PID in its .raioz.state.json (see routeOwnerAliveOnHost).
// Without this, a project that crashed without running `raioz down` leaves
// an immortal route file that pins the shared proxy and injects a dead
// backend into every Caddyfile reload. See ADR-005 (orphan route-file GC).
//
// Docker-unreachable guard: if the liveness probe fails we cannot prove any
// file is an orphan, so we skip the GC entirely (degrading to the pre-issue
// keep-alive behaviour) rather than risk deleting routes of a project that
// is actually running.
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
		// Launcher-pattern services (`command: make start` → `docker
		// compose up -d`) daemonize: the recorded host PID dies at once
		// and the resulting containers are user-owned (no raioz labels),
		// so both probes above miss them. The persisted route targets are
		// the durable link to the real backends — any of them running is
		// proof of life (ADR-005, same fallback as sibling_probe ADR-008).
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

// routeOwnerAliveOnHost reports whether the project owning a route file is
// alive host-side. A project whose services run via `command:` (host process
// or user-owned container) has no raioz-labeled containers, so container
// absence alone is not proof of death — its route file records the project
// directory, and that directory's .raioz.state.json records host PIDs.
//
// The pruning contract (ADR-005) is "positive proof of death, never absence
// of proof of life": returns true (keep the file) whenever liveness cannot
// be determined — legacy or remote (ADR-049) route files without a
// projectDir, or an unreadable state file. Returns false only when the
// state is readable and shows no live host PIDs.
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

// routeTargetAliveInDocker reports whether any container-name target in the
// project's persisted route file is currently running. It is the third
// liveness probe of the orphan-route GC, covering launcher-pattern services
// whose user-owned containers carry no raioz labels and whose recorded host
// PID (the launcher) is already dead — the two earlier probes miss them.
//
// Returns false when the file has no container targets (no signal; the
// earlier probes already decided). Fail-closed: a docker probe error keeps
// the file (returns true), matching ADR-005's "never absence of proof of
// life".
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
// Containers with an empty project label are shared deps or the proxy
// itself (ADR-002). A live shared dep vetoes the teardown: by ADR-002's
// own contract deps survive individual downs until the last consumer
// leaves, so one still running means some consumer remains — possibly a
// project whose services run via `command:` (host process or user-owned
// container) and therefore owns no labeled containers at all. The current
// project's own shared deps can't false-positive here: the orchestrated
// down stops them before stopProxy runs.
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
// still up. Two probes, ORed: raioz-labeled containers (the historical signal,
// including ADR-002's shared-dep veto) and, for launcher-pattern projects that
// own no labeled container at all, a running container among some sibling's
// persisted route targets (ADR-005 rule 3).
//
// The second probe only changes outcomes under --all / --prune-shared: without
// force, a surviving route file already keeps the proxy alive on its own. With
// force, it is what stops the teardown from cutting the HTTPS out from under a
// `command: make start` sibling.
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
