package app

import (
	"testing"
	"time"

	"raioz/internal/mocks"
	"raioz/internal/state"
)

func newTestDepsForList(t *testing.T) (*Dependencies, *mocks.MockStateManager) {
	t.Helper()

	stateMgr := &mocks.MockStateManager{}

	deps := &Dependencies{
		ConfigLoader:  &mocks.MockConfigLoader{},
		Workspace:     &mocks.MockWorkspaceManager{},
		StateManager:  stateMgr,
		DockerRunner:  &mocks.MockDockerRunner{},
		Validator:     &mocks.MockValidator{},
		GitRepository: &mocks.MockGitRepository{},
		LockManager:   &mocks.MockLockManager{},
		HostRunner:    &mocks.MockHostRunner{},
		EnvManager:    &mocks.MockEnvManager{},
	}

	return deps, stateMgr
}

func TestListUseCase_Execute_NoState(t *testing.T) {
	deps, stateMgr := newTestDepsForList(t)

	stateMgr.LoadGlobalStateFunc = func() (*state.GlobalState, error) {
		return &state.GlobalState{
			ActiveProjects: []string{},
			Projects:       map[string]state.ProjectState{},
		}, nil
	}

	uc := NewListUseCase(deps)
	err := uc.Execute(ListOptions{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListUseCase_Execute_WithProjects(t *testing.T) {
	deps, stateMgr := newTestDepsForList(t)

	stateMgr.LoadGlobalStateFunc = func() (*state.GlobalState, error) {
		return &state.GlobalState{
			ActiveProjects: []string{"project-a", "project-b"},
			Projects: map[string]state.ProjectState{
				"project-a": {
					Name:          "project-a",
					Workspace:     "project-a",
					LastExecution: time.Now().Add(-5 * time.Minute),
					Services: []state.ServiceState{
						{Name: "api", Mode: "dev", Status: "running"},
						{Name: "web", Mode: "dev", Status: "running"},
					},
				},
				"project-b": {
					Name:          "project-b",
					Workspace:     "project-b",
					LastExecution: time.Now().Add(-1 * time.Hour),
					Services: []state.ServiceState{
						{Name: "worker", Mode: "prod", Status: "stopped"},
					},
				},
			},
		}, nil
	}

	uc := NewListUseCase(deps)
	err := uc.Execute(ListOptions{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListUseCase_Execute_JSONOutput(t *testing.T) {
	deps, stateMgr := newTestDepsForList(t)

	stateMgr.LoadGlobalStateFunc = func() (*state.GlobalState, error) {
		return &state.GlobalState{
			ActiveProjects: []string{"project-a"},
			Projects: map[string]state.ProjectState{
				"project-a": {
					Name:      "project-a",
					Workspace: "project-a",
					Services: []state.ServiceState{
						{Name: "api", Mode: "dev", Status: "running"},
					},
				},
			},
		}, nil
	}

	uc := NewListUseCase(deps)
	err := uc.Execute(ListOptions{JSONOutput: true})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListUseCase_ApplyFilters(t *testing.T) {
	deps, _ := newTestDepsForList(t)
	uc := NewListUseCase(deps)

	globalState := &state.GlobalState{
		ActiveProjects: []string{"my-api", "my-web", "other"},
		Projects: map[string]state.ProjectState{
			"my-api": {
				Name: "my-api",
				Services: []state.ServiceState{
					{Name: "api", Status: "running"},
				},
			},
			"my-web": {
				Name: "my-web",
				Services: []state.ServiceState{
					{Name: "web", Status: "stopped"},
				},
			},
			"other": {
				Name: "other",
				Services: []state.ServiceState{
					{Name: "worker", Status: "running"},
				},
			},
		},
	}

	tests := []struct {
		name     string
		opts     ListOptions
		expected int
	}{
		{
			name:     "no filter returns all",
			opts:     ListOptions{},
			expected: 3,
		},
		{
			name:     "filter by name prefix",
			opts:     ListOptions{Filter: "my-"},
			expected: 2,
		},
		{
			name:     "filter by status running",
			opts:     ListOptions{Status: "running"},
			expected: 2,
		},
		{
			name:     "filter by name and status",
			opts:     ListOptions{Filter: "my-", Status: "running"},
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := uc.applyFilters(globalState, tt.opts)
			if len(result.ActiveProjects) != tt.expected {
				t.Errorf("expected %d active projects, got %d", tt.expected, len(result.ActiveProjects))
			}
		})
	}
}
