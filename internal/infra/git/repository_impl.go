package git

import (
	"context"
	"raioz/internal/config"
	"raioz/internal/domain/interfaces"
	gitpkg "raioz/internal/git"
)

// Ensure GitRepositoryImpl implements interfaces.GitRepository
var _ interfaces.GitRepository = (*GitRepositoryImpl)(nil)

// GitRepositoryImpl is the concrete implementation of GitRepository
type GitRepositoryImpl struct{}

// NewGitRepository creates a new GitRepository implementation
func NewGitRepository() interfaces.GitRepository {
	return &GitRepositoryImpl{}
}

// EnsureRepo ensures a repository exists and is up to date
func (r *GitRepositoryImpl) EnsureRepo(src config.SourceConfig, baseDir string) error {
	return gitpkg.EnsureRepo(src, baseDir)
}

// EnsureRepoWithForce ensures a repository exists, with option to force re-clone
func (r *GitRepositoryImpl) EnsureRepoWithForce(src config.SourceConfig, baseDir string, force bool) error {
	return gitpkg.EnsureRepoWithForce(src, baseDir, force)
}

// EnsureReadonlyRepo ensures a readonly repository exists without updating it
func (r *GitRepositoryImpl) EnsureReadonlyRepo(src config.SourceConfig, baseDir string) error {
	return gitpkg.EnsureReadonlyRepo(src, baseDir)
}

// EnsureEditableRepo ensures an editable repository exists and is up to date
func (r *GitRepositoryImpl) EnsureEditableRepo(src config.SourceConfig, baseDir string) error {
	return gitpkg.EnsureEditableRepo(src, baseDir)
}

// ForceReclone removes the repository directory and clones it fresh (with context)
func (r *GitRepositoryImpl) ForceReclone(ctx context.Context, repoPath string, repo string, branch string) error {
	return gitpkg.ForceReclone(ctx, repoPath, repo, branch)
}

// UpdateReposIfBranchChanged updates repositories if their branches changed
func (r *GitRepositoryImpl) UpdateReposIfBranchChanged(
	ctx context.Context,
	repoPathResolver func(string, config.Service) string,
	oldDeps, newDeps *config.Deps,
) error {
	return gitpkg.UpdateReposIfBranchChanged(ctx, repoPathResolver, oldDeps, newDeps)
}

// IsReadonly checks if a source configuration is readonly
func (r *GitRepositoryImpl) IsReadonly(src config.SourceConfig) bool {
	return gitpkg.IsReadonly(src)
}
