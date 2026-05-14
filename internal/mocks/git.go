package mocks

import (
	"context"

	"raioz/internal/domain/interfaces"
	"raioz/internal/domain/models"
)

// Compile-time check
var _ interfaces.GitRepository = (*MockGitRepository)(nil)

// MockGitRepository is a mock implementation of interfaces.GitRepository
type MockGitRepository struct {
	EnsureRepoFunc                 func(src models.SourceConfig, baseDir string) error
	EnsureRepoWithForceFunc        func(src models.SourceConfig, baseDir string, force bool) error
	EnsureReadonlyRepoFunc         func(src models.SourceConfig, baseDir string) error
	EnsureEditableRepoFunc         func(src models.SourceConfig, baseDir string) error
	ForceRecloneFunc               func(ctx context.Context, repoPath string, repo string, branch string) error
	UpdateReposIfBranchChangedFunc func(
		ctx context.Context,
		repoPathResolver func(string, models.Service) string,
		oldDeps, newDeps *models.Deps,
	) error
	IsReadonlyFunc func(src models.SourceConfig) bool
}

func (m *MockGitRepository) EnsureRepo(src models.SourceConfig, baseDir string) error {
	if m.EnsureRepoFunc != nil {
		return m.EnsureRepoFunc(src, baseDir)
	}
	return nil
}

func (m *MockGitRepository) EnsureRepoWithForce(src models.SourceConfig, baseDir string, force bool) error {
	if m.EnsureRepoWithForceFunc != nil {
		return m.EnsureRepoWithForceFunc(src, baseDir, force)
	}
	return nil
}

func (m *MockGitRepository) EnsureReadonlyRepo(src models.SourceConfig, baseDir string) error {
	if m.EnsureReadonlyRepoFunc != nil {
		return m.EnsureReadonlyRepoFunc(src, baseDir)
	}
	return nil
}

func (m *MockGitRepository) EnsureEditableRepo(src models.SourceConfig, baseDir string) error {
	if m.EnsureEditableRepoFunc != nil {
		return m.EnsureEditableRepoFunc(src, baseDir)
	}
	return nil
}

func (m *MockGitRepository) ForceReclone(ctx context.Context, repoPath string, repo string, branch string) error {
	if m.ForceRecloneFunc != nil {
		return m.ForceRecloneFunc(ctx, repoPath, repo, branch)
	}
	return nil
}

func (m *MockGitRepository) UpdateReposIfBranchChanged(
	ctx context.Context,
	repoPathResolver func(string, models.Service) string,
	oldDeps, newDeps *models.Deps,
) error {
	if m.UpdateReposIfBranchChangedFunc != nil {
		return m.UpdateReposIfBranchChangedFunc(ctx, repoPathResolver, oldDeps, newDeps)
	}
	return nil
}

func (m *MockGitRepository) IsReadonly(src models.SourceConfig) bool {
	if m.IsReadonlyFunc != nil {
		return m.IsReadonlyFunc(src)
	}
	return false
}
