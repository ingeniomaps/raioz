package app

import (
	"fmt"
	"os"
	"testing"
	"time"

	"raioz/internal/config"
	"raioz/internal/mocks"
	"raioz/internal/state"
	"raioz/internal/workspace"
)

func TestFormatTime(t *testing.T) {
	tests := []struct {
		name string
		time time.Time
		want string
	}{
		{"zero", time.Time{}, "never"},
		{"just now", time.Now().Add(-10 * time.Second), "just now"},
		{"minutes ago", time.Now().Add(-15 * time.Minute), "15 minute(s) ago"},
		{"hours ago", time.Now().Add(-3 * time.Hour), "3 hour(s) ago"},
		{"days ago", time.Now().Add(-48 * time.Hour), "2 day(s) ago"},
		{"old date", time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), "2020-01-01 00:00:00"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatTime(tt.time)
			if got != tt.want {
				t.Errorf("formatTime(%v) = %q, want %q", tt.time, got, tt.want)
			}
		})
	}
}

func TestProcessAlive_CurrentProcess(t *testing.T) {
	if !processAlive(os.Getpid()) {
		t.Error("expected current process to be alive")
	}
}

func TestProcessAlive_InvalidPID(t *testing.T) {
	// PID 0 on Linux sends signals to every process in the group, which
	// is not what we want. Use a very high PID that's extremely unlikely to exist.
	if processAlive(999999999) {
		t.Error("expected non-existent PID to not be alive")
	}
}

func TestListUseCase_LoadHostPIDs_EmptyWorkspace(t *testing.T) {
	deps, _ := newTestDepsForList(t)
	uc := NewListUseCase(deps)
	result := uc.loadHostPIDs("")
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

func TestListUseCase_LoadHostPIDs_ResolveError(t *testing.T) {
	deps, _ := newTestDepsForList(t)
	deps.Workspace = &mocks.MockWorkspaceManager{
		ResolveFunc: func(name string) (*workspace.Workspace, error) {
			return nil, fmt.Errorf("fail")
		},
	}
	uc := NewListUseCase(deps)
	result := uc.loadHostPIDs("test")
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

func TestListUseCase_LoadHostPIDs_FromProjectRoot(t *testing.T) {
	tmpDir := t.TempDir()
	// Save local state with PIDs
	ls := &state.LocalState{
		HostPIDs: map[string]int{"api": os.Getpid()},
	}
	_ = state.SaveLocalState(tmpDir, ls)

	deps, _ := newTestDepsForList(t)
	deps.Workspace = &mocks.MockWorkspaceManager{
		ResolveFunc: func(name string) (*workspace.Workspace, error) {
			return &workspace.Workspace{Root: tmpDir}, nil
		},
		GetRootFunc: func(ws *workspace.Workspace) string { return tmpDir },
	}
	deps.StateManager = &mocks.MockStateManager{
		LoadFunc: func(ws *workspace.Workspace) (*config.Deps, error) {
			return &config.Deps{
				ProjectRoot: tmpDir,
			}, nil
		},
	}
	uc := NewListUseCase(deps)
	result := uc.loadHostPIDs("test")
	if result == nil {
		t.Fatal("expected non-nil PIDs map")
	}
	if _, ok := result["api"]; !ok {
		t.Error("expected 'api' in PIDs map")
	}
}

func TestListUseCase_LoadHostPIDs_FallbackToWsRoot(t *testing.T) {
	tmpDir := t.TempDir()
	// Save local state at workspace root
	ls := &state.LocalState{
		HostPIDs: map[string]int{"worker": 12345},
	}
	_ = state.SaveLocalState(tmpDir, ls)

	deps, _ := newTestDepsForList(t)
	deps.Workspace = &mocks.MockWorkspaceManager{
		ResolveFunc: func(name string) (*workspace.Workspace, error) {
			return &workspace.Workspace{Root: tmpDir}, nil
		},
		GetRootFunc: func(ws *workspace.Workspace) string { return tmpDir },
	}
	deps.StateManager = &mocks.MockStateManager{
		LoadFunc: func(ws *workspace.Workspace) (*config.Deps, error) {
			return nil, fmt.Errorf("no state")
		},
	}
	uc := NewListUseCase(deps)
	result := uc.loadHostPIDs("test")
	if result == nil {
		t.Fatal("expected non-nil PIDs from workspace root")
	}
}

func TestListUseCase_Execute_LoadError(t *testing.T) {
	initI18nForTest(t)
	deps, stateMgr := newTestDepsForList(t)
	stateMgr.LoadGlobalStateFunc = func() (*state.GlobalState, error) {
		return nil, fmt.Errorf("load fail")
	}
	uc := NewListUseCase(deps)
	err := uc.Execute(ListOptions{})
	if err == nil {
		t.Fatal("expected error for load failure")
	}
}

func TestListUseCase_Execute_FilterByStatus(t *testing.T) {
	initI18nForTest(t)
	deps, stateMgr := newTestDepsForList(t)
	stateMgr.LoadGlobalStateFunc = func() (*state.GlobalState, error) {
		return &state.GlobalState{
			ActiveProjects: []string{"a", "b"},
			Projects: map[string]state.ProjectState{
				"a": {Name: "a", Services: []state.ServiceState{{Name: "s1", Status: "running"}}},
				"b": {Name: "b", Services: []state.ServiceState{{Name: "s2", Status: "stopped"}}},
			},
		}, nil
	}
	uc := NewListUseCase(deps)
	err := uc.Execute(ListOptions{Status: "stopped"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
