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
			// Drop only this project's reference and keep the dep up while
			// any other project still references it. We trust the refcount,
			// not a container scan: a sibling that consumes only shared deps
			// owns no project-labeled container, so scanning would wrongly
			// read it as gone and rip the dep out from under it (issue 069).
			remaining, err := refcount.DropRef(deps.Workspace, name, projectName)
			if err != nil {
				logging.WarnWithContext(ctx, "Shared dep refcount drop failed",
					"dep", name, "error", err.Error())
			}
			if len(remaining) > 0 {
				logging.InfoWithContext(ctx, "Keeping shared dependency alive; still referenced",
					"dep", name, "workspace", deps.Workspace,
					"leaving_project", projectName, "remaining", remaining)
				continue
			}
		}

		projName := naming.DepComposeProjectNameFor(projectName, name, naming.IsSharedDep(override))
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
