package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"raioz/internal/docker"
	"raioz/internal/domain/interfaces"
	"raioz/internal/domain/models"
	"raioz/internal/i18n"
	"raioz/internal/logging"
	"raioz/internal/naming"
	"raioz/internal/output"
	"raioz/internal/state"
)

// stopProxy tears down the Caddy proxy (or just removes this project's
// contribution from a shared one) when raioz down runs.
//
// Per-project mode (legacy, no workspace): unconditional Stop.
//
// Workspace-shared mode:
//  1. Remove this project's persisted routes file.
//  2. If at least one project remains in the workspace, regenerate the
//     Caddyfile and Reload — the proxy keeps serving the surviving
//     projects' routes without our entries.
//  3. If we were the last project, Stop the shared proxy (and let cleanup
//     of the routes dir happen naturally on next reboot via /tmp).
func (uc *DownUseCase) stopProxy(ctx context.Context, opts DownOptions) {
	if uc.deps.ProxyManager == nil {
		return
	}

	deps, _, _ := uc.deps.ConfigLoader.LoadDeps(opts.ConfigPath)
	if deps == nil {
		return
	}
	// ADR-013 Phase 2 / ADR-032: configure via a single Configure
	// call rather than per-field setters. We only need to pin the
	// project + workspace scope so the manager can locate the right
	// container / routes file; the other fields keep their defaults.
	uc.deps.ProxyManager.Configure(interfaces.ProxyConfig{
		ProjectName: deps.Project.Name,
		Workspace:   deps.Workspace,
	})

	if deps.Workspace != "" {
		uc.handleSharedProxyDown(ctx, deps)
		return
	}

	uc.handlePerProjectProxyDown(ctx)
	cleanProxyDirOnDisk(ctx, deps)
}

// handleSharedProxyDown implements the workspace-shared lifecycle: drop our
// routes, then either reload (siblings remain) or stop (last one out).
func (uc *DownUseCase) handleSharedProxyDown(ctx context.Context, deps *models.Deps) {
	if err := uc.deps.ProxyManager.RemoveProjectRoutes(); err != nil {
		logging.WarnWithContext(ctx, "Failed to remove project routes",
			"project", deps.Project.Name, "error", err.Error())
	}

	// Garbage-collect route files left behind by projects that crashed (or
	// were torn down outside raioz) without running their own `down`. Such
	// an orphan file makes RemainingProjects() > 0 forever, pinning the
	// shared proxy alive on every down even when we are the last project
	// out (ADR-005). Must run before the gate below so RemainingProjects
	// reflects only genuinely-live projects.
	uc.pruneOrphanRouteFiles(ctx, deps.Workspace, deps.Project.Name)

	// Two independent signals must agree before we tumba: no persisted
	// routes for any project AND no other workspace-labeled containers
	// still alive. Either alone is too aggressive — routes can be stale
	// from a crash, and labels can be stale during a partial up — but
	// requiring both keeps siblings safe in the common case.
	noRouteFiles := uc.deps.ProxyManager.RemainingProjects() == 0
	noLiveSiblings := !otherWorkspaceProjectsActive(ctx, deps.Workspace, deps.Project.Name)

	if !noRouteFiles || !noLiveSiblings {
		// Reload so the proxy stops serving our removed routes.
		if err := uc.deps.ProxyManager.Reload(ctx); err != nil {
			logging.WarnWithContext(ctx, "Failed to reload shared proxy after route removal",
				"workspace", deps.Workspace, "error", err.Error())
		}
		logging.InfoWithContext(ctx, "Keeping shared proxy alive for sibling projects",
			"workspace", deps.Workspace, "leaving_project", deps.Project.Name)
		return
	}

	uc.handlePerProjectProxyDown(ctx)
	cleanProxyDirOnDisk(ctx, deps)
}

// handlePerProjectProxyDown is the legacy "just stop the container" path.
// Used both for projects without a workspace and for the last project
// leaving a workspace. The on-disk proxy dir cleanup is left to the caller
// because only it knows which dir applies (workspace-shared vs per-project).
func (uc *DownUseCase) handlePerProjectProxyDown(ctx context.Context) {
	running, err := uc.deps.ProxyManager.Status(ctx)
	if err != nil || !running {
		return
	}
	output.PrintInfo(i18n.T("output.stopping_proxy"))
	if err := uc.deps.ProxyManager.Stop(ctx); err != nil {
		logging.WarnWithContext(ctx, "Failed to stop proxy", "error", err.Error())
		output.PrintWarning(i18n.T("warning.proxy_stop_failed", err.Error()))
	} else {
		output.PrintSuccess(i18n.T("output.proxy_stopped"))
	}
}

// cleanProxyDirOnDisk removes the on-disk Caddyfile + routes dir that the
// proxy left behind. The proxy container's bind-mount source becomes garbage
// once the container is gone — without this the next `up` of an unrelated
// project in the same workspace would inherit the previous project's routes
// until raioz overwrote them. Workspace-shared mode targets WorkspaceProxyDir;
// legacy per-project mode targets ProxyDir(project).
//
// Legacy migration: also nuke the pre-XDG `/tmp/<ws>/proxy/`
// location. Users upgrading from a build that wrote there inherit a stale
// (and possibly root-owned) tree; one down/up cycle now clears it. The
// legacy removal is best-effort because the offending tree is exactly the
// kind that the upgrading user can't os.RemoveAll without sudo — log and
// move on instead of failing the down.
func cleanProxyDirOnDisk(ctx context.Context, deps *models.Deps) {
	var current, legacy string
	if deps.Workspace != "" {
		current = naming.WorkspaceProxyDir()
		legacy = naming.LegacyWorkspaceProxyDir()
	} else {
		current = naming.ProxyDir(deps.Project.Name)
		legacy = naming.LegacyProxyDir(deps.Project.Name)
	}

	for _, dir := range []string{current, legacy} {
		if dir == "" {
			continue
		}
		if err := os.RemoveAll(dir); err != nil && !os.IsNotExist(err) {
			logging.WarnWithContext(ctx, "Failed to remove proxy dir",
				"dir", dir, "error", err.Error())
		}
	}
}

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

// pruneOrphanRouteFiles removes persisted route files whose owning project
// has no live container in the workspace. Without this, a project that
// crashed without running `raioz down` leaves an immortal route file that
// pins the shared proxy and injects a dead backend into every Caddyfile
// reload. See ADR-005 (orphan route-file GC).
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
		if err := uc.deps.ProxyManager.RemoveRoutesFor(proj); err != nil {
			logging.WarnWithContext(ctx, "Failed to prune orphan route file",
				"workspace", workspace, "project", proj, "error", err.Error())
			continue
		}
		logging.InfoWithContext(ctx, "Pruned orphan route file (project has no live containers)",
			"workspace", workspace, "project", proj)
	}
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
		if proj == "" {
			continue // shared dep or shared proxy — not a project consumer
		}
		if proj != currentProject {
			return true
		}
	}
	return false
}

// cleanLocalState removes the .raioz.state.json from the project directory.
func (uc *DownUseCase) cleanLocalState(ctx context.Context, opts DownOptions) {
	if opts.ConfigPath == "" {
		return
	}

	projectDir, err := filepath.Abs(filepath.Dir(opts.ConfigPath))
	if err != nil {
		return
	}

	if err := state.RemoveLocalState(projectDir); err != nil {
		logging.WarnWithContext(ctx, "Failed to remove local state", "error", err.Error())
	}
}
