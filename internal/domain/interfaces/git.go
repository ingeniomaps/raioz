package interfaces

import (
	"context"
	models "raioz/internal/domain/models"
)

// GitRepository defines operations for Git repository management
type GitRepository interface {
	// EnsureRepo ensures a repository exists and is up to date
	EnsureRepo(src models.SourceConfig, baseDir string) error
	// EnsureRepoWithForce ensures a repository exists, with option to force re-clone
	EnsureRepoWithForce(src models.SourceConfig, baseDir string, force bool) error
	// EnsureReadonlyRepo ensures a readonly repository exists without updating it
	EnsureReadonlyRepo(src models.SourceConfig, baseDir string) error
	// EnsureEditableRepo ensures an editable repository exists and is up to date
	EnsureEditableRepo(src models.SourceConfig, baseDir string) error
	// ForceReclone removes the repository directory and clones it fresh (with context)
	ForceReclone(ctx context.Context, repoPath string, repo string, branch string) error
	// UpdateReposIfBranchChanged updates repos if branches changed
	UpdateReposIfBranchChanged(
		ctx context.Context,
		repoPathResolver func(string, models.Service) string,
		oldDeps, newDeps *models.Deps,
	) error
	// IsReadonly checks if a source configuration is readonly
	IsReadonly(src models.SourceConfig) bool
}
