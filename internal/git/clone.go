package git

import (
	"fmt"
	"path/filepath"

	"raioz/internal/config"
	exectimeout "raioz/internal/exec"
	"raioz/internal/logging"
)

// EnsureRepo ensures a repository exists and is up to date
// It uses EnsureReadonlyRepo or EnsureEditableRepo based on the access field
func EnsureRepo(src config.SourceConfig, baseDir string) error {
	if IsReadonly(src) {
		return EnsureReadonlyRepo(src, baseDir)
	}
	return EnsureEditableRepo(src, baseDir)
}

// EnsureRepoWithForce allows forcing a re-clone of the repository
// Note: Force re-clone is only allowed for editable repos
func EnsureRepoWithForce(src config.SourceConfig, baseDir string, force bool) error {
	if force && IsReadonly(src) {
		return fmt.Errorf(
			"cannot force re-clone readonly repository '%s'. "+
				"Readonly repositories are protected from forced updates",
			src.Path,
		)
	}

	target := filepath.Join(baseDir, src.Path)

	if force {
		// Force re-clone (only for editable repos)
		logging.Info("Force re-cloning repository", "path", src.Path)
		ctx, cancel := exectimeout.WithTimeout(exectimeout.GitCloneTimeout)
		defer cancel()
		if err := ForceReclone(ctx, target, src.Repo, src.Branch); err != nil {
			return fmt.Errorf("failed to force re-clone repository: %w", err)
		}
		return nil
	}

	// Normal EnsureRepo behavior
	return EnsureRepo(src, baseDir)
}
