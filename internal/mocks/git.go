package mocks

import "raioz/internal/config"

type GitClient struct {
	EnsureRepoFunc func(src config.SourceConfig, baseDir string) error
}

func (m *GitClient) EnsureRepo(src config.SourceConfig, baseDir string) error {
	if m.EnsureRepoFunc != nil {
		return m.EnsureRepoFunc(src, baseDir)
	}
	return nil
}
