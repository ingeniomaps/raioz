package interfaces

import (
	"context"
	"raioz/internal/config"
)

// GitRepository defines operations for Git repository management
type GitRepository interface {
	// EnsureRepo ensures a repository exists and is up to date
	EnsureRepo(src config.SourceConfig, baseDir string) error
	// EnsureRepoWithForce ensures a repository exists, with option to force re-clone
	EnsureRepoWithForce(src config.SourceConfig, baseDir string, force bool) error
	// EnsureReadonlyRepo ensures a readonly repository exists without updating it
	EnsureReadonlyRepo(src config.SourceConfig, baseDir string) error
	// EnsureEditableRepo ensures an editable repository exists and is up to date
	EnsureEditableRepo(src config.SourceConfig, baseDir string) error
	// ForceReclone removes the repository directory and clones it fresh (with context)
	ForceReclone(ctx context.Context, repoPath string, repo string, branch string) error
	// UpdateReposIfBranchChanged updates repositories if their branches changed
	UpdateReposIfBranchChanged(ctx context.Context, repoPathResolver func(string, config.Service) string, oldDeps, newDeps *config.Deps) error
	// IsReadonly checks if a source configuration is readonly
	IsReadonly(src config.SourceConfig) bool
}
