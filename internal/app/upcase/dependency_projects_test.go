package upcase

import (
	"context"
	stderrors "errors"
	"path/filepath"
	"strings"
	"testing"

	"raioz/internal/domain/models"
	"raioz/internal/mocks"
)

// stopRunningProject stopped executing anything the day .raioz.json
// stopped loading. What it still owes the caller is the deregistration.
func TestStopRunningProjectDeregisters(t *testing.T) {
	initI18nUp(t)

	removed := ""
	uc := NewUseCase(&Dependencies{
		StateManager: &mocks.MockStateManager{
			RemoveProjectFunc: func(name string) error {
				removed = name
				return nil
			},
		},
	})

	err := uc.stopRunningProject(context.Background(), "myproj",
		models.ProjectState{Name: "myproj", Workspace: "/nonexistent/path"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if removed != "myproj" {
		t.Errorf("removed %q from global state, want %q", removed, "myproj")
	}
}

func TestStopRunningProjectSurfacesRemoveFailure(t *testing.T) {
	initI18nUp(t)

	uc := NewUseCase(&Dependencies{
		StateManager: &mocks.MockStateManager{
			RemoveProjectFunc: func(string) error { return stderrors.New("state locked") },
		},
	})

	err := uc.stopRunningProject(context.Background(), "myproj",
		models.ProjectState{Name: "myproj", Workspace: "/nonexistent/path"})
	if err == nil {
		t.Fatal("expected the deregistration failure to surface")
	}
	if !strings.Contains(err.Error(), "state locked") {
		t.Errorf("error = %v, want it to carry the cause", err)
	}
}

// --- stopRunningProject ------------------------------------------------------

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

	ps := models.ProjectState{Name: "proj", Workspace: "/tmp"}
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

	ps := models.ProjectState{Name: "proj", Workspace: "/tmp"}
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
			LoadGlobalStateFunc: func() (*models.GlobalState, error) {
				return &models.GlobalState{
					Projects: map[string]models.ProjectState{
						"db": {
							Name:      "db",
							Workspace: "/tmp/db",
							Services:  []models.ServiceState{}, // Empty = command-based
						},
					},
				}, nil
			},
			GetGlobalStatePathFunc: func() (string, error) { return gsPath, nil },
		},
	})

	deps := &models.Deps{
		Project: models.Project{Name: "p"},
		Services: map[string]models.Service{
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
			LoadGlobalStateFunc: func() (*models.GlobalState, error) {
				return &models.GlobalState{
					Projects: map[string]models.ProjectState{
						"redis": {
							Name:      "redis",
							Workspace: "/tmp/redis",
							Services:  []models.ServiceState{},
						},
					},
				}, nil
			},
			GetGlobalStatePathFunc: func() (string, error) { return gsPath, nil },
		},
	})

	deps := &models.Deps{
		Project: models.Project{Name: "p"},
		Services: map[string]models.Service{
			"api": {
				DependsOn: []string{"redis"},
				Docker:    &models.DockerConfig{DependsOn: []string{"redis"}},
			},
		},
	}

	err := uc.checkDependencyProjects(context.Background(), deps)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
