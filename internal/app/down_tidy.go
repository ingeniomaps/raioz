package app

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"

	"raioz/internal/domain/models"
	"raioz/internal/logging"
	"raioz/internal/naming"
	"raioz/internal/root"
	"raioz/internal/state"
)

// tidyAfterDown removes what a fully stopped project leaves on disk and
// nothing reads again: the generated dependency compose files, a workspace
// directory that holds no file, and a local state file that records
// nothing the next `up` needs. Each step is best-effort — a leftover is
// untidy, not an error.
//
// The generated compose files go even when a shared dependency this project
// started keeps serving another one: its eventual teardown runs
// `docker compose -p <project> down`, which needs no file.
func tidyAfterDown(
	ctx context.Context, projectName, projectDir, workspaceDir string,
	localState *models.LocalState,
) {
	if projectName != "" {
		if err := os.RemoveAll(naming.TempDir(projectName)); err != nil {
			logging.WarnWithContext(ctx, "Failed to remove generated dependency files",
				"project", projectName, "error", err.Error())
		}
	}

	removeIfHoldsNoFile(workspaceDir)

	if localState != nil && localStateIsDisposable(localState) {
		if err := state.RemoveLocalState(projectDir); err != nil {
			logging.WarnWithContext(ctx, "Failed to remove local state",
				"project", projectName, "error", err.Error())
		}
	}
}

// localStateIsDisposable reports whether a stopped project's state file
// carries anything Docker and raioz.yaml cannot give back. A dependency
// promoted with `raioz dev` and a service ignored by hand are choices the
// user made; everything else is rewritten by the next `up`.
func localStateIsDisposable(s *models.LocalState) bool {
	return len(s.DevOverrides) == 0 && len(s.Ignored) == 0 && len(s.HostPIDs) == 0
}

// removeIfHoldsNoFile deletes dir when its whole tree is directories only.
// One regular file anywhere keeps all of it.
func removeIfHoldsNoFile(dir string) {
	if dir == "" {
		return
	}
	holdsFile := false
	err := filepath.WalkDir(dir, func(_ string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !d.IsDir() {
			holdsFile = true
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil || holdsFile {
		return
	}
	_ = os.RemoveAll(dir)
}

// dropWorkspaceState removes the project's state once it is fully down
// (ADR-023): `raioz.root.json`, plus what tidyAfterDown covers.
//
// The directory is the one `up` wrote to — named after the workspace when
// the project declares one, not after the project. Projects of a workspace
// share it, so it is left alone while another of them still has
// containers; the project's own leftovers go either way.
func (uc *DownUseCase) dropWorkspaceState(
	ctx context.Context, deps *models.Deps, projectName, projectDir string,
	localState *models.LocalState,
) {
	shared := deps.Workspace != "" && otherWorkspaceProjectsActive(ctx, deps.Workspace, projectName)
	if shared {
		tidyAfterDown(ctx, projectName, projectDir, "", localState)
		return
	}
	ws, err := uc.deps.Workspace.Resolve(deps.GetWorkspaceName())
	switch {
	case err != nil:
		logging.WarnWithContext(ctx, "Skipping root cleanup: workspace resolve failed",
			"project", projectName, "error", err.Error())
		return
	case ws == nil:
		// Mocks (and lenient real impls) can return (nil, nil).
		return
	}
	if err := root.Delete(ws); err != nil {
		logging.WarnWithContext(ctx, "Failed to remove root config",
			"project", projectName, "error", err.Error())
	}
	tidyAfterDown(ctx, projectName, projectDir, uc.deps.Workspace.GetRoot(ws), localState)
}
