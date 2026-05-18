package app

import (
	"context"
	"os/exec"
	"path/filepath"

	"raioz/internal/docker"
	"raioz/internal/domain/models"
	"raioz/internal/logging"
	"raioz/internal/naming"
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
	others := otherProjectsActiveInWorkspace(ctx, deps.Workspace, projectName)
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
		if naming.IsSharedDep(override) && others {
			logging.InfoWithContext(ctx, "Keeping shared dependency alive for sibling projects",
				"dep", name, "workspace", deps.Workspace, "leaving_project", projectName)
			continue
		}

		projName := naming.DepComposeProjectName(projectName, name)
		// Compose-based deps: user-supplied fragment(s) + raioz overlay,
		// teardown needs the same list of -f files that Start used.
		var composeArgs []string
		var envFileArgs []string
		if entry.Inline != nil && len(entry.Inline.Compose) > 0 {
			overlay := filepath.Join(
				filepath.Dir(naming.DepComposePath(projectName, name)),
				"raioz-overlay.yml",
			)
			for _, f := range entry.Inline.Compose {
				abs := f
				if a, err := filepath.Abs(f); err == nil {
					abs = a
				}
				composeArgs = append(composeArgs, "-f", abs)
			}
			composeArgs = append(composeArgs, "-f", overlay)
			if entry.Inline.Env != nil {
				for _, f := range entry.Inline.Env.GetFilePaths() {
					if f != "" {
						envFileArgs = append(envFileArgs, "--env-file", f)
					}
				}
			}
		} else {
			composeArgs = []string{"-f", naming.DepComposePath(projectName, name)}
		}
		args := []string{"compose"}
		args = append(args, envFileArgs...)
		args = append(args, "-p", projName)
		args = append(args, composeArgs...)
		args = append(args, "down")
		cmd := exec.CommandContext(ctx, runtime.Binary(), args...)
		_ = cmd.Run() // file might not exist; best-effort teardown
	}
}

// otherProjectsActiveInWorkspace answers the "is anyone else home?" question
// needed to decide whether shared deps can be torn down. Returns true when at
// least one raioz-managed container in the workspace belongs to a project
// other than the one currently being brought down. Shared deps themselves
// have no project label, so they don't falsely signal other-project activity.
func otherProjectsActiveInWorkspace(ctx context.Context, workspace, currentProject string) bool {
	if workspace == "" {
		return false
	}
	names := docker.ListContainersByLabels(ctx, map[string]string{
		naming.LabelManaged:   "true",
		naming.LabelWorkspace: workspace,
	})
	for _, n := range names {
		proj, _ := docker.GetContainerLabel(ctx, n, naming.LabelProject)
		if proj == "" {
			continue // a shared dep itself — not a project consumer
		}
		if proj != currentProject {
			return true
		}
	}
	return false
}
