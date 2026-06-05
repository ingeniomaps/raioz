package app

import (
	"context"
	"os/exec"

	"raioz/internal/domain/models"
	"raioz/internal/logging"
	"raioz/internal/naming"
	"raioz/internal/refcount"
	"raioz/internal/runtime"
)

// stopDependencyComposeProjects stops compose projects created by image_runner.
// Uses the same COMPOSE_PROJECT_NAME that ImageRunner.Start set so Docker
// Compose can match the containers it originally created.
//
// Shared dependencies (workspace-scoped or with an explicit `name:` override)
// are skipped while OTHER raioz projects in the same workspace still have
// live containers — the last project out tumba the shared deps. Without
// this guard, project A's down would rip postgres out from under project B.
//
// `deferredDeps` lists dep names whose dispatch was skipped at up time
// because a sibling raioz project owns them (issue #26 mode B). Those
// have no container in the consumer's namespace and must not be torn
// down — running `docker compose down` for them would be a no-op but
// still spawns a process per dep, so we filter early. Pass nil for
// legacy projects without sibling deps.
func stopDependencyComposeProjects(
	ctx context.Context,
	deps *models.Deps,
	projectName string,
	deferredDeps []string,
) {
	// Reconcile the shared-dep refcount against the projects that are
	// actually live (excluding this one, which is fully leaving). This
	// drops our own references and purges any left behind by a sibling
	// that died without a clean `down`, so the per-dep keep-alive check
	// below trusts a count that mirrors reality (issue 069).
	live := liveProjectsInWorkspace(ctx, deps.Workspace, projectName)
	if err := refcount.Reconcile(deps.Workspace, live); err != nil {
		logging.WarnWithContext(ctx, "Shared dep refcount reconcile failed",
			"workspace", deps.Workspace, "error", err.Error())
	}

	deferred := make(map[string]struct{}, len(deferredDeps))
	for _, n := range deferredDeps {
		deferred[n] = struct{}{}
	}

	for name, entry := range deps.Infra {
		// Mode A (project:) — sibling-owned dep with its own lifecycle.
		// We never started a container for it and must never stop one.
		if entry.Inline != nil && entry.Inline.Project != "" {
			logging.InfoWithContext(ctx, "Skipping mode A sibling dep on down",
				"dep", name, "sibling", entry.Inline.Project)
			continue
		}
		// Mode B with deferred-to-sibling stamp from up time.
		if _, isDeferred := deferred[name]; isDeferred {
			logging.InfoWithContext(ctx, "Skipping deferred sibling dep on down",
				"dep", name)
			continue
		}

		var override string
		if entry.Inline != nil {
			override = entry.Inline.Name
		}
		if naming.IsSharedDep(override) {
			refs, err := refcount.Refs(deps.Workspace, name)
			if err != nil {
				logging.WarnWithContext(ctx, "Shared dep refcount lookup failed",
					"dep", name, "error", err.Error())
			} else if len(refs) > 0 {
				logging.InfoWithContext(ctx, "Keeping shared dependency alive; still referenced",
					"dep", name, "workspace", deps.Workspace,
					"leaving_project", projectName, "remaining", refs)
				continue
			}
		}

		projName := naming.DepComposeProjectName(projectName, name)
		// Tear down by `-p` alone: docker compose resolves the project from
		// the labels the engine stamped at up time, so it doesn't need the
		// original -f fragments (which live under TMPDIR and may be gone in
		// a later session, a cleaned /tmp, or another host). Reconstructing
		// the -f list and swallowing the error left deps leaking silently.
		// --remove-orphans sweeps any container still carrying the label.
		args := []string{"compose", "-p", projName, "down", "--remove-orphans"}
		cmd := exec.CommandContext(ctx, runtime.Binary(), args...)
		if out, err := cmd.CombinedOutput(); err != nil {
			logging.WarnWithContext(ctx, "Dependency teardown failed",
				"dep", name, "project", projName,
				"error", err.Error(), "output", string(out))
		}
	}
}

// liveProjectsInWorkspace returns the distinct project names (other than
// currentProject) that still have at least one raioz-managed container up
// in the workspace. Shared deps carry no project label, so they never show
// up as consumers. This is the authoritative "who is actually live" set the
// refcount reconciler trusts over its own persisted entries (issue 069).
func liveProjectsInWorkspace(ctx context.Context, workspace, currentProject string) []string {
	if workspace == "" {
		return nil
	}
	names := listContainersByLabelsFn(ctx, map[string]string{
		naming.LabelManaged:   "true",
		naming.LabelWorkspace: workspace,
	})
	seen := map[string]struct{}{}
	var live []string
	for _, n := range names {
		proj, _ := getContainerLabelFn(ctx, n, naming.LabelProject)
		if proj == "" || proj == currentProject {
			continue
		}
		if _, ok := seen[proj]; !ok {
			seen[proj] = struct{}{}
			live = append(live, proj)
		}
	}
	return live
}

// anyLive reports whether any of refs names a project in the live set. Used
// to ignore stale references (a project that died without dropping its ref)
// when deciding whether a shared dep still has a real consumer.
func anyLive(refs, live []string) bool {
	set := make(map[string]struct{}, len(live))
	for _, p := range live {
		set[p] = struct{}{}
	}
	for _, r := range refs {
		if _, ok := set[r]; ok {
			return true
		}
	}
	return false
}
