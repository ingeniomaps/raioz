package app

import (
	"context"
	"os"
	"path/filepath"

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
		// --all and --prune-shared mean the same thing to the proxy.
		uc.handleSharedProxyDown(ctx, deps, opts.All || opts.PruneShared)
		return
	}

	uc.handlePerProjectProxyDown(ctx)
	cleanProxyDirOnDisk(ctx, deps)
}

// handleSharedProxyDown implements the workspace-shared lifecycle: drop our
// routes, then either reload (siblings remain) or stop (last one out).
//
// force waives the route-file half of the gate so a leftover file can't pin
// the proxy forever. The live-sibling half is never waived: no flag tears
// down a proxy another project is still serving traffic through.
func (uc *DownUseCase) handleSharedProxyDown(ctx context.Context, deps *models.Deps, force bool) {
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

	// Both signals must agree before we tumba: routes can be stale from a
	// crash and labels can be stale during a partial up, so either alone is
	// too aggressive. `force` drops the route half — the stale-file case.
	noRouteFiles := uc.deps.ProxyManager.RemainingProjects() == 0
	noLiveSiblings := !uc.workspaceSiblingAlive(ctx, deps.Workspace, deps.Project.Name)

	if (!force && !noRouteFiles) || !noLiveSiblings {
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
