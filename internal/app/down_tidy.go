package app

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"

	"raioz/internal/domain/models"
	"raioz/internal/logging"
	"raioz/internal/naming"
	"raioz/internal/state"
)

// tidyAfterDown removes what a fully stopped project leaves on disk and
// nothing reads again: the generated dependency compose files, a workspace
// directory that holds no file, and a local state file that records
// nothing the next `up` needs. Each step is best-effort — a leftover is
// untidy, not an error.
//
// keepDepFiles is true while a shared dependency this project started is
// still serving another one: its compose file lives in this project's
// temp tree and its eventual `down` needs it.
func tidyAfterDown(
	ctx context.Context, projectName, projectDir, workspaceDir string,
	localState *models.LocalState, keepDepFiles bool,
) {
	if projectName != "" && !keepDepFiles {
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
