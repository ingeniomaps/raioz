package app

import (
	"context"
	"testing"

	"raioz/internal/domain/interfaces"
	"raioz/internal/mocks"
)

func newTestDepsForPorts(t *testing.T) (*Dependencies, *mocks.MockWorkspaceManager, *mocks.MockDockerRunner) {
	t.Helper()

	tmpDir := t.TempDir()

	wsMgr := &mocks.MockWorkspaceManager{
		GetBaseDirFunc: func() (string, error) {
			return tmpDir, nil
		},
	}
	dockerRunner := &mocks.MockDockerRunner{}

	deps := &Dependencies{
		ConfigLoader:  &mocks.MockConfigLoader{},
		Workspace:     wsMgr,
		StateManager:  &mocks.MockStateManager{},
		DockerRunner:  dockerRunner,
		Validator:     &mocks.MockValidator{},
		GitRepository: &mocks.MockGitRepository{},
		LockManager:   &mocks.MockLockManager{},
		HostRunner:    &mocks.MockHostRunner{},
		EnvManager:    &mocks.MockEnvManager{},
	}

	return deps, wsMgr, dockerRunner
}

func TestPortsUseCase_Execute_NoPorts(t *testing.T) {
	deps, _, dockerRunner := newTestDepsForPorts(t)

	dockerRunner.GetAllActivePortsFunc = func(baseDir string) ([]interfaces.PortInfo, error) {
		return []interfaces.PortInfo{}, nil
	}

	uc := NewPortsUseCase(deps)
	err := uc.Execute(context.Background(), PortsOptions{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPortsUseCase_Execute_WithPorts(t *testing.T) {
	deps, _, dockerRunner := newTestDepsForPorts(t)

	dockerRunner.GetAllActivePortsFunc = func(baseDir string) ([]interfaces.PortInfo, error) {
		return []interfaces.PortInfo{
			{Port: "8080:80", Project: "test-project", Service: "api"},
			{Port: "5432:5432", Project: "test-project", Service: "postgres"},
		}, nil
	}

	uc := NewPortsUseCase(deps)
	err := uc.Execute(context.Background(), PortsOptions{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
