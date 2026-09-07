package app

import (
	"context"

	"raioz/internal/i18n"
	"raioz/internal/logging"
)

// pruneStaleProjectStates drops global-state entries for projects that are
// no longer running.
//
// `up` writes an entry per project and, until 2026-09, nothing removed it:
// the map grew one entry per project ever started and kept it after the
// directory was deleted. `raioz down` now deregisters on a complete
// teardown, which stops the growth but does not undo the history — 81
// entries on the maintainer's machine, 15 of them for scratch projects.
//
// Liveness is the criterion, not age: an entry earns its place by having
// containers. A probe that fails keeps the entry, because "the daemon did
// not answer" and "the project is gone" must not lead to the same
// deletion — the same rule classifyRefs follows for shared-dep refs.
//
// Only runs under `--all`, where the user asked to tidy everything rather
// than one project.
func (uc *CleanUseCase) pruneStaleProjectStates(ctx context.Context, dryRun bool) []string {
	globalState, err := uc.deps.StateManager.LoadGlobalState()
	if err != nil || globalState == nil {
		if err != nil {
			logging.WarnWithContext(ctx, "Skipping global-state prune: load failed",
				"error", err.Error())
		}
		return nil
	}

	var actions []string
	for name, project := range globalState.Projects {
		active, probeErr := uc.deps.DockerRunner.IsProjectActive(ctx, project.Workspace, name)
		if probeErr != nil {
			logging.WarnWithContext(ctx, "Liveness probe failed; keeping project state",
				"project", name, "workspace", project.Workspace, "error", probeErr.Error())
			continue
		}
		if active {
			continue
		}

		if dryRun {
			actions = append(actions, i18n.T("clean.would_deregister_project", name))
			continue
		}
		if err := uc.deps.StateManager.RemoveProject(name); err != nil {
			logging.WarnWithContext(ctx, "Failed to deregister project",
				"project", name, "error", err.Error())
			continue
		}
		actions = append(actions, i18n.T("clean.deregistered_project", name))
	}
	return actions
}
