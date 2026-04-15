package app

import (
	"os"
	"testing"

	"raioz/internal/i18n"
	"raioz/internal/mocks"
)

// initI18nForTest initializes i18n in English for tests.
func initI18nForTest(t *testing.T) {
	t.Helper()
	os.Setenv("RAIOZ_LANG", "en")
	t.Cleanup(func() { os.Unsetenv("RAIOZ_LANG") })
	i18n.Init("en")
}

// newFullMockDeps returns a Dependencies with all mock fields populated but
// no Func fields wired. Individual tests override the Func fields they need.
func newFullMockDeps() *Dependencies {
	return &Dependencies{
		ConfigLoader:  &mocks.MockConfigLoader{},
		Workspace:     &mocks.MockWorkspaceManager{},
		StateManager:  &mocks.MockStateManager{},
		DockerRunner:  &mocks.MockDockerRunner{},
		Validator:     &mocks.MockValidator{},
		GitRepository: &mocks.MockGitRepository{},
		LockManager:   &mocks.MockLockManager{},
		HostRunner:    &mocks.MockHostRunner{},
		EnvManager:    &mocks.MockEnvManager{},
	}
}
