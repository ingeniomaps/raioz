package app

import (
	"context"
	"strings"
	"testing"

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

func stubActiveEndpoints(t *testing.T, endpoints []activeEndpoint) {
	t.Helper()
	prev := activeEndpointsFn
	activeEndpointsFn = func(context.Context) []activeEndpoint { return endpoints }
	t.Cleanup(func() { activeEndpointsFn = prev })
}

func TestPortsUseCase_Execute_NoPorts(t *testing.T) {
	initI18nForTest(t)
	deps, _, _ := newTestDepsForPorts(t)
	stubActiveEndpoints(t, nil)

	if err := NewPortsUseCase(deps).Execute(context.Background(), PortsOptions{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// `raioz ports` lists every port a raioz project holds, whichever way the
// thing behind it was launched: a container and a host process both count.
func TestPortsUseCase_Execute_ListsBothRunners(t *testing.T) {
	initI18nForTest(t)
	deps, _, _ := newTestDepsForPorts(t)
	stubActiveEndpoints(t, []activeEndpoint{
		{Project: "bencha", Service: "web", Runner: runnerHost, Port: 38101},
		{Project: "bencha", Service: "cache", Runner: runnerContainer, Port: 36379},
		{Workspace: "acme", Service: "postgres", Runner: runnerContainer, Port: 5432},
		{Project: "other", Service: "api", Runner: runnerHost, Port: 3000},
	})

	out := captureStdout(t, func() {
		if err := NewPortsUseCase(deps).Execute(context.Background(), PortsOptions{}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	for _, want := range []string{"38101", "web", runnerHost, "36379", runnerContainer, "acme", "3000"} {
		if !strings.Contains(out, want) {
			t.Errorf("output misses %q:\n%s", want, out)
		}
	}

	filtered := captureStdout(t, func() {
		_ = NewPortsUseCase(deps).Execute(context.Background(), PortsOptions{ProjectName: "bencha"})
	})
	if strings.Contains(filtered, "3000") || !strings.Contains(filtered, "38101") {
		t.Errorf("-p bencha should list only bencha's ports:\n%s", filtered)
	}
}
