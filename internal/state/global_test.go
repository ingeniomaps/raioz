package state

import (
	"testing"
	"time"

	"raioz/internal/config"
)

// setupGlobalHome creates a temp dir and sets RAIOZ_HOME so global state writes
// go to an isolated location.
func setupGlobalHome(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("RAIOZ_HOME", dir)
}

func TestGetGlobalStatePath(t *testing.T) {
	setupGlobalHome(t)
	path, err := GetGlobalStatePath()
	if err != nil {
		t.Fatalf("GetGlobalStatePath: %v", err)
	}
	if path == "" {
		t.Error("expected non-empty path")
	}
}

func TestLoadGlobalState_Empty(t *testing.T) {
	setupGlobalHome(t)

	// No state file exists yet — should return empty state
	state, err := LoadGlobalState()
	if err != nil {
		t.Fatalf("LoadGlobalState: %v", err)
	}
	if state == nil {
		t.Fatal("expected non-nil state")
	}
	if len(state.ActiveProjects) != 0 {
		t.Errorf("expected 0 active projects, got %d", len(state.ActiveProjects))
	}
	if state.Projects == nil {
		t.Error("Projects map should be initialized")
	}
}

func TestSaveAndLoadGlobalState(t *testing.T) {
	setupGlobalHome(t)

	state := &GlobalState{
		ActiveProjects: []string{"proj1"},
		Projects: map[string]ProjectState{
			"proj1": {
				Name:          "proj1",
				Workspace:     "ws1",
				LastExecution: time.Now(),
				Services: []ServiceState{
					{Name: "api", Mode: "dev", Status: "running"},
				},
			},
		},
	}

	if err := SaveGlobalState(state); err != nil {
		t.Fatalf("SaveGlobalState: %v", err)
	}

	loaded, err := LoadGlobalState()
	if err != nil {
		t.Fatalf("LoadGlobalState: %v", err)
	}

	if len(loaded.ActiveProjects) != 1 || loaded.ActiveProjects[0] != "proj1" {
		t.Errorf("active projects mismatch: %v", loaded.ActiveProjects)
	}
	if ps, ok := loaded.Projects["proj1"]; !ok {
		t.Error("proj1 not loaded")
	} else if len(ps.Services) != 1 {
		t.Errorf("expected 1 service, got %d", len(ps.Services))
	}
}

func TestGetActiveProjects(t *testing.T) {
	setupGlobalHome(t)

	state := &GlobalState{
		ActiveProjects: []string{"a", "b"},
		Projects:       map[string]ProjectState{},
	}
	if err := SaveGlobalState(state); err != nil {
		t.Fatalf("SaveGlobalState: %v", err)
	}

	active, err := GetActiveProjects()
	if err != nil {
		t.Fatalf("GetActiveProjects: %v", err)
	}
	if len(active) != 2 {
		t.Errorf("expected 2, got %d", len(active))
	}
}

func TestGetProjectState_Found(t *testing.T) {
	setupGlobalHome(t)

	state := &GlobalState{
		ActiveProjects: []string{"p1"},
		Projects: map[string]ProjectState{
			"p1": {Name: "p1"},
		},
	}
	SaveGlobalState(state)

	got, err := GetProjectState("p1")
	if err != nil {
		t.Fatalf("GetProjectState: %v", err)
	}
	if got.Name != "p1" {
		t.Errorf("got %+v", got)
	}
}

func TestGetProjectState_NotFound(t *testing.T) {
	setupGlobalHome(t)

	_, err := GetProjectState("nonexistent")
	if err == nil {
		t.Error("expected error")
	}
}

func TestUpdateProjectState_New(t *testing.T) {
	setupGlobalHome(t)

	ps := ProjectState{
		Name:      "newproj",
		Workspace: "ws",
	}
	if err := UpdateProjectState("newproj", ps); err != nil {
		t.Fatalf("UpdateProjectState: %v", err)
	}

	state, _ := LoadGlobalState()
	if _, ok := state.Projects["newproj"]; !ok {
		t.Error("project not added")
	}

	// Should also be in active list
	found := false
	for _, name := range state.ActiveProjects {
		if name == "newproj" {
			found = true
		}
	}
	if !found {
		t.Error("project not in ActiveProjects")
	}
}

func TestUpdateProjectState_Idempotent(t *testing.T) {
	setupGlobalHome(t)

	ps := ProjectState{Name: "p1"}
	_ = UpdateProjectState("p1", ps)
	_ = UpdateProjectState("p1", ps)

	state, _ := LoadGlobalState()
	// ActiveProjects should contain p1 only once
	count := 0
	for _, name := range state.ActiveProjects {
		if name == "p1" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected 1 entry for p1 in ActiveProjects, got %d", count)
	}
}

func TestRemoveProject(t *testing.T) {
	setupGlobalHome(t)

	_ = UpdateProjectState("p1", ProjectState{Name: "p1"})
	_ = UpdateProjectState("p2", ProjectState{Name: "p2"})

	if err := RemoveProject("p1"); err != nil {
		t.Fatalf("RemoveProject: %v", err)
	}

	state, _ := LoadGlobalState()
	if _, ok := state.Projects["p1"]; ok {
		t.Error("p1 should be removed from Projects")
	}
	if _, ok := state.Projects["p2"]; !ok {
		t.Error("p2 should still exist")
	}

	// Check ActiveProjects
	for _, name := range state.ActiveProjects {
		if name == "p1" {
			t.Error("p1 should not be in ActiveProjects")
		}
	}
}

func TestUpdateLastExecution_New(t *testing.T) {
	setupGlobalHome(t)

	if err := UpdateLastExecution("newproj"); err != nil {
		t.Fatalf("UpdateLastExecution: %v", err)
	}

	got, err := GetProjectState("newproj")
	if err != nil {
		t.Fatalf("GetProjectState: %v", err)
	}
	if got.LastExecution.IsZero() {
		t.Error("expected non-zero LastExecution")
	}
}

func TestUpdateLastExecution_Existing(t *testing.T) {
	setupGlobalHome(t)

	_ = UpdateProjectState("p1", ProjectState{
		Name:          "p1",
		LastExecution: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
	})

	if err := UpdateLastExecution("p1"); err != nil {
		t.Fatalf("UpdateLastExecution: %v", err)
	}

	got, _ := GetProjectState("p1")
	if got.LastExecution.Year() == 2020 {
		t.Error("LastExecution should have been updated")
	}
}

func TestBuildServiceStates_Empty(t *testing.T) {
	deps := &config.Deps{
		Services: map[string]config.Service{},
	}
	got := BuildServiceStates(deps, nil)
	if len(got) != 0 {
		t.Errorf("expected empty, got %d", len(got))
	}
}

func TestBuildServiceStates_WithServices(t *testing.T) {
	deps := &config.Deps{
		Services: map[string]config.Service{
			"api": {
				Source: config.SourceConfig{
					Kind:  "image",
					Image: "myapi",
					Tag:   "v1",
				},
				Docker: &config.DockerConfig{
					Mode: "prod",
				},
			},
			"worker": {
				Source: config.SourceConfig{
					Kind: "git",
				},
			},
		},
	}

	got := BuildServiceStates(deps, nil)
	if len(got) != 2 {
		t.Errorf("expected 2 services, got %d", len(got))
	}

	for _, s := range got {
		if s.Name == "api" {
			if s.Mode != "prod" {
				t.Errorf("api mode: got %q, want prod", s.Mode)
			}
			if s.Image != "myapi:v1" {
				t.Errorf("api image: got %q", s.Image)
			}
		}
		if s.Name == "worker" && s.Mode != "dev" {
			t.Errorf("worker mode: got %q, want dev (default)", s.Mode)
		}
	}
}

func TestBuildServiceStates_WithInfo(t *testing.T) {
	deps := &config.Deps{
		Services: map[string]config.Service{
			"api": {},
		},
	}
	infos := map[string]*ServiceInfo{
		"api": {
			Status:  "running",
			Version: "abc123",
			Image:   "myapi",
		},
	}

	got := BuildServiceStates(deps, infos)
	if len(got) != 1 {
		t.Fatalf("expected 1, got %d", len(got))
	}
	if got[0].Status != "running" {
		t.Errorf("expected status=running, got %q", got[0].Status)
	}
	if got[0].Version != "abc123" {
		t.Errorf("expected version=abc123, got %q", got[0].Version)
	}
}
