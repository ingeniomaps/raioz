package app

import (
	"context"
	"path/filepath"

	"raioz/internal/config"
	"raioz/internal/docker"
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
	uc.deps.ProxyManager.SetProjectName(deps.Project.Name)
	uc.deps.ProxyManager.SetWorkspace(deps.Workspace)

	if deps.Workspace != "" {
		uc.handleSharedProxyDown(ctx, deps)
		return
	}

	uc.handlePerProjectProxyDown(ctx)
}

// handleSharedProxyDown implements the workspace-shared lifecycle: drop our
// routes, then either reload (siblings remain) or stop (last one out).
func (uc *DownUseCase) handleSharedProxyDown(ctx context.Context, deps *config.Deps) {
	if err := uc.deps.ProxyManager.RemoveProjectRoutes(); err != nil {
		logging.WarnWithContext(ctx, "Failed to remove project routes",
			"project", deps.Project.Name, "error", err.Error())
	}

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
}

// handlePerProjectProxyDown is the legacy "just stop the container" path.
// Used both for projects without a workspace and for the last project
// leaving a workspace.
func (uc *DownUseCase) handlePerProjectProxyDown(ctx context.Context) {
	running, err := uc.deps.ProxyManager.Status(ctx)
	if err != nil || !running {
		return
	}
	output.PrintInfo("Stopping proxy...")
	if err := uc.deps.ProxyManager.Stop(ctx); err != nil {
		logging.WarnWithContext(ctx, "Failed to stop proxy", "error", err.Error())
		output.PrintWarning("Failed to stop proxy: " + err.Error())
	} else {
		output.PrintSuccess("Proxy stopped")
	}
}

// listContainersByLabelsFn / getContainerLabelFn are package-level hooks for
// the workspace-occupancy probe. Production points at docker.* directly;
// tests stub them so they can simulate any mix of sibling presence without
// a real Docker daemon.
var listContainersByLabelsFn = docker.ListContainersByLabels
var getContainerLabelFn = docker.GetContainerLabel

// otherWorkspaceProjectsActive mirrors otherProjectsActiveInWorkspace from
// down_orchestrated.go but lives here to avoid a package-internal cycle.
// Returns true when at least one raioz-managed container in the workspace
// belongs to a project other than the one currently being torn down.
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
