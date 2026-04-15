package upcase

import (
	"context"
	stderrors "errors"
	"os"
	"path/filepath"
	"testing"

	"raioz/internal/config"
	"raioz/internal/mocks"
	"raioz/internal/state"
)

// --- stopRunningProject ------------------------------------------------------

func TestStopRunningProjectConfigNotFound(t *testing.T) {
	initI18nUp(t)

	removeCalled := false
	uc := NewUseCase(&Dependencies{
		StateManager: &mocks.MockStateManager{
			RemoveProjectFunc: func(name string) error {
				removeCalled = true
				return nil
			},
		},
	})

	// Use non-existent workspace directory
	ps := state.ProjectState{
		Name:      "myproj",
		Workspace: "/nonexistent/path",
	}

	err := uc.stopRunningProject(context.Background(), "myproj", ps)
	if err == nil {
		t.Error("expected error when config file not found")
	}
	if !removeCalled {
		t.Error("should still remove from global state even if config not found")
	}
}

func TestStopRunningProjectLoadConfigError(t *testing.T) {
	initI18nUp(t)

	dir := t.TempDir()
	// Create a .raioz.json file so os.Stat succeeds
	configPath := filepath.Join(dir, ".raioz.json")
	if err := os.WriteFile(configPath, []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	removeCalled := false
	uc := NewUseCase(&Dependencies{
		ConfigLoader: &mocks.MockConfigLoader{
			LoadDepsFunc: func(path string) (*config.Deps, []string, error) {
				return nil, nil, stderrors.New("parse error")
			},
		},
		StateManager: &mocks.MockStateManager{
			RemoveProjectFunc: func(name string) error {
				removeCalled = true
				return nil
			},
		},
	})

	ps := state.ProjectState{
		Name:      "myproj",
		Workspace: dir,
	}

	err := uc.stopRunningProject(context.Background(), "myproj", ps)
	if err == nil {
		t.Error("expected error when config load fails")
	}
	if !removeCalled {
		t.Error("should still remove from global state on config load error")
	}
}

func TestStopRunningProjectSuccess(t *testing.T) {
	initI18nUp(t)

	dir := t.TempDir()
	raiozHome := t.TempDir()
	t.Setenv("RAIOZ_HOME", raiozHome)

	// Create a .raioz.json file
	configPath := filepath.Join(dir, ".raioz.json")
	if err := os.WriteFile(configPath, []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	// Put the dir inside RAIOZ_HOME/workspaces to make it NOT local
	wsDir := filepath.Join(raiozHome, "workspaces", "test")
	if err := os.MkdirAll(wsDir, 0755); err != nil {
		t.Fatal(err)
	}

	removeCalled := false
	uc := NewUseCase(&Dependencies{
		ConfigLoader: &mocks.MockConfigLoader{
			LoadDepsFunc: func(path string) (*config.Deps, []string, error) {
				return &config.Deps{
					Project: config.Project{Name: "myproj"},
				}, nil, nil
			},
		},
		StateManager: &mocks.MockStateManager{
			RemoveProjectFunc: func(name string) error {
				removeCalled = true
				return nil
			},
		},
	})

	ps := state.ProjectState{
		Name:      "myproj",
		Workspace: dir,
	}

	err := uc.stopRunningProject(context.Background(), "myproj", ps)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !removeCalled {
		t.Error("RemoveProject should be called")
	}
}

// --- askReplaceRunningProject with saved decisions ---------------------------

func TestAskReplaceRunningProjectSavedDecisionTrue(t *testing.T) {
	initI18nUp(t)

	dir := t.TempDir()
	gsPath := filepath.Join(dir, "state.json")
	sm := &mocks.MockStateManager{
		GetGlobalStatePathFunc: func() (string, error) { return gsPath, nil },
	}

	// Save a decision first
	if err := recordUserDecision("proj", true, sm); err != nil {
		t.Fatal(err)
	}

	ps := state.ProjectState{Name: "proj", Workspace: "/tmp"}
	shouldReplace, err := askReplaceRunningProject(context.Background(), "proj", ps, sm)
	if err != nil {
		t.Fatal(err)
	}
	if !shouldReplace {
		t.Error("expected true from saved decision")
	}
}

func TestAskReplaceRunningProjectSavedDecisionFalse(t *testing.T) {
	initI18nUp(t)

	dir := t.TempDir()
	gsPath := filepath.Join(dir, "state.json")
	sm := &mocks.MockStateManager{
		GetGlobalStatePathFunc: func() (string, error) { return gsPath, nil },
	}

	// Save a decision first
	if err := recordUserDecision("proj", false, sm); err != nil {
		t.Fatal(err)
	}

	ps := state.ProjectState{Name: "proj", Workspace: "/tmp"}
	shouldReplace, err := askReplaceRunningProject(context.Background(), "proj", ps, sm)
	if err != nil {
		t.Fatal(err)
	}
	if shouldReplace {
		t.Error("expected false from saved decision")
	}
}

// --- checkDependencyProjects with matching command-based project ---------------

func TestCheckDependencyProjectsMatchingCommandProject(t *testing.T) {
	initI18nUp(t)

	dir := t.TempDir()
	gsPath := filepath.Join(dir, "state.json")

	// Pre-save decision to avoid stdin read
	sm := &mocks.MockStateManager{
		GetGlobalStatePathFunc: func() (string, error) { return gsPath, nil },
	}
	if err := recordUserDecision("db", false, sm); err != nil {
		t.Fatal(err)
	}

	uc := NewUseCase(&Dependencies{
		StateManager: &mocks.MockStateManager{
			LoadGlobalStateFunc: func() (*state.GlobalState, error) {
				return &state.GlobalState{
					Projects: map[string]state.ProjectState{
						"db": {
							Name:      "db",
							Workspace: "/tmp/db",
							Services:  []state.ServiceState{}, // Empty = command-based
						},
					},
				}, nil
			},
			GetGlobalStatePathFunc: func() (string, error) { return gsPath, nil },
		},
	})

	deps := &config.Deps{
		Project: config.Project{Name: "p"},
		Services: map[string]config.Service{
			"api": {DependsOn: []string{"db"}},
		},
	}

	err := uc.checkDependencyProjects(context.Background(), deps)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- checkDependencyProjects with docker depends-on --------------------------

func TestCheckDependencyProjectsDockerDependsOn(t *testing.T) {
	initI18nUp(t)

	dir := t.TempDir()
	gsPath := filepath.Join(dir, "state.json")
	sm := &mocks.MockStateManager{
		GetGlobalStatePathFunc: func() (string, error) { return gsPath, nil },
	}
	if err := recordUserDecision("redis", false, sm); err != nil {
		t.Fatal(err)
	}

	uc := NewUseCase(&Dependencies{
		StateManager: &mocks.MockStateManager{
			LoadGlobalStateFunc: func() (*state.GlobalState, error) {
				return &state.GlobalState{
					Projects: map[string]state.ProjectState{
						"redis": {
							Name:      "redis",
							Workspace: "/tmp/redis",
							Services:  []state.ServiceState{},
						},
					},
				}, nil
			},
			GetGlobalStatePathFunc: func() (string, error) { return gsPath, nil },
		},
	})

	deps := &config.Deps{
		Project: config.Project{Name: "p"},
		Services: map[string]config.Service{
			"api": {
				DependsOn: []string{"redis"},
				Docker:    &config.DockerConfig{DependsOn: []string{"redis"}},
			},
		},
	}

	err := uc.checkDependencyProjects(context.Background(), deps)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
