package mocks

import (
	"context"

	"raioz/internal/config"
	"raioz/internal/domain/interfaces"
)

// Compile-time check
var _ interfaces.GitRepository = (*MockGitRepository)(nil)

// MockGitRepository is a mock implementation of interfaces.GitRepository
type MockGitRepository struct {
	EnsureRepoFunc              func(src config.SourceConfig, baseDir string) error
	EnsureRepoWithForceFunc     func(src config.SourceConfig, baseDir string, force bool) error
	EnsureReadonlyRepoFunc      func(src config.SourceConfig, baseDir string) error
	EnsureEditableRepoFunc      func(src config.SourceConfig, baseDir string) error
	ForceRecloneFunc            func(ctx context.Context, repoPath string, repo string, branch string) error
	UpdateReposIfBranchChangedFunc func(ctx context.Context, repoPathResolver func(string, config.Service) string, oldDeps, newDeps *config.Deps) error
	IsReadonlyFunc              func(src config.SourceConfig) bool
}

func (m *MockGitRepository) EnsureRepo(src config.SourceConfig, baseDir string) error {
	if m.EnsureRepoFunc != nil {
		return m.EnsureRepoFunc(src, baseDir)
	}
	return nil
}

func (m *MockGitRepository) EnsureRepoWithForce(src config.SourceConfig, baseDir string, force bool) error {
	if m.EnsureRepoWithForceFunc != nil {
		return m.EnsureRepoWithForceFunc(src, baseDir, force)
	}
	return nil
}

func (m *MockGitRepository) EnsureReadonlyRepo(src config.SourceConfig, baseDir string) error {
	if m.EnsureReadonlyRepoFunc != nil {
		return m.EnsureReadonlyRepoFunc(src, baseDir)
	}
	return nil
}

func (m *MockGitRepository) EnsureEditableRepo(src config.SourceConfig, baseDir string) error {
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

func (m *MockGitRepository) UpdateReposIfBranchChanged(ctx context.Context, repoPathResolver func(string, config.Service) string, oldDeps, newDeps *config.Deps) error {
	if m.UpdateReposIfBranchChangedFunc != nil {
		return m.UpdateReposIfBranchChangedFunc(ctx, repoPathResolver, oldDeps, newDeps)
	}
	return nil
}

func (m *MockGitRepository) IsReadonly(src config.SourceConfig) bool {
	if m.IsReadonlyFunc != nil {
		return m.IsReadonlyFunc(src)
	}
	return false
}
