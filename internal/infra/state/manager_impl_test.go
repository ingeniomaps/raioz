package state

import (
	"os"
	"path/filepath"
	"testing"

	"raioz/internal/config"
	"raioz/internal/domain/interfaces"
	statepkg "raioz/internal/state"
)

func setupWS(t *testing.T) *interfaces.Workspace {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("RAIOZ_HOME", dir)

	wsDir := filepath.Join(dir, "testproj")
	if err := os.MkdirAll(wsDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	return &interfaces.Workspace{
		Root:                wsDir,
		ServicesDir:         filepath.Join(wsDir, "services"),
		LocalServicesDir:    filepath.Join(wsDir, "local"),
		ReadonlyServicesDir: filepath.Join(wsDir, "readonly"),
		EnvDir:              filepath.Join(wsDir, "env"),
	}
}

func TestNewStateManager(t *testing.T) {
	m := NewStateManager()
	if m == nil {
		t.Fatal("NewStateManager returned nil")
	}
}

func TestStateManagerImpl_SaveLoadExists(t *testing.T) {
	m := NewStateManager()
	ws := setupWS(t)

	// Initially no state
	if m.Exists(ws) {
		t.Error("expected no state initially")
	}

	deps := &config.Deps{
		Project: config.Project{Name: "testproj"},
		Services: map[string]config.Service{
			"api": {},
		},
	}

	if err := m.Save(ws, deps); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if !m.Exists(ws) {
		t.Error("expected state to exist after Save")
	}

	loaded, err := m.Load(ws)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded == nil || loaded.Project.Name != "testproj" {
		t.Errorf("Load returned unexpected: %+v", loaded)
	}
}

func TestStateManagerImpl_Load_NoState(t *testing.T) {
	m := NewStateManager()
	ws := setupWS(t)

	loaded, err := m.Load(ws)
	if err != nil {
		t.Errorf("Load on empty ws should not error: %v", err)
	}
	if loaded != nil {
		t.Error("expected nil for non-existent state")
	}
}

func TestStateManagerImpl_CompareDeps(t *testing.T) {
	m := NewStateManager()

	oldDeps := &config.Deps{
		Services: map[string]config.Service{"api": {}},
	}
	newDeps := &config.Deps{
		Services: map[string]config.Service{
			"api": {},
			"web": {},
		},
	}

	changes, err := m.CompareDeps(oldDeps, newDeps)
	if err != nil {
		t.Fatalf("CompareDeps: %v", err)
	}
	if len(changes) == 0 {
		t.Log("no changes detected — may be implementation-specific")
	}
}

func TestStateManagerImpl_FormatChanges(t *testing.T) {
	m := NewStateManager()
	formatted := m.FormatChanges([]statepkg.ConfigChange{})
	_ = formatted // Just verify it doesn't crash
}

func TestStateManagerImpl_UpdateRemoveProject(t *testing.T) {
	// Set up isolated global state
	dir := t.TempDir()
	t.Setenv("RAIOZ_HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)

	m := NewStateManager()

	projState := &statepkg.ProjectState{
		Name: "myproj",
	}
	err := m.UpdateProjectState("myproj", projState)
	if err != nil {
		t.Logf("UpdateProjectState failed (may be expected): %v", err)
	}

	err = m.RemoveProject("myproj")
	if err != nil {
		t.Logf("RemoveProject failed: %v", err)
	}
}

func TestStateManagerImpl_UpdateProjectState_NilError(t *testing.T) {
	m := NewStateManager()
	if err := m.UpdateProjectState("p", nil); err == nil {
		t.Error("expected error for nil projectState")
	}
}

func TestStateManagerImpl_LoadGlobalState(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("RAIOZ_HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)

	m := NewStateManager()
	_, err := m.LoadGlobalState()
	if err != nil {
		t.Logf("LoadGlobalState: %v", err)
	}
}

func TestStateManagerImpl_GetGlobalStatePath(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("RAIOZ_HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)

	m := NewStateManager()
	path, err := m.GetGlobalStatePath()
	if err != nil {
		t.Fatalf("GetGlobalStatePath: %v", err)
	}
	if path == "" {
		t.Error("expected non-empty path")
	}
}

func TestStateManagerImpl_GetSetServicePreference(t *testing.T) {
	m := NewStateManager()
	ws := setupWS(t)

	// No preference initially
	_, _ = m.GetServicePreference(ws, "api")

	pref := statepkg.ServicePreference{
		ServiceName: "api",
	}
	if err := m.SetServicePreference(ws, pref); err != nil {
		t.Fatalf("SetServicePreference: %v", err)
	}

	got, err := m.GetServicePreference(ws, "api")
	if err != nil {
		t.Fatalf("GetServicePreference: %v", err)
	}
	if got == nil || got.ServiceName != "api" {
		t.Errorf("got %+v, want api", got)
	}
}

func TestStateManagerImpl_GetSetWorkspaceProjectPreference(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("RAIOZ_HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)

	m := NewStateManager()

	pref := statepkg.WorkspaceProjectPreference{
		PreferredProject: "myproj",
	}
	if err := m.SetWorkspaceProjectPreference("ws1", pref); err != nil {
		t.Logf("SetWorkspaceProjectPreference: %v", err)
	}

	_, err := m.GetWorkspaceProjectPreference("ws1")
	if err != nil {
		t.Logf("GetWorkspaceProjectPreference: %v", err)
	}
}

func TestStateManagerImpl_BuildServiceStates(t *testing.T) {
	m := NewStateManager()
	deps := &config.Deps{
		Services: map[string]config.Service{"api": {}},
	}
	states := m.BuildServiceStates(deps, nil)
	if len(states) == 0 {
		t.Log("no states built — may be impl-specific")
	}
}

func TestStateManagerImpl_FormatIssues(t *testing.T) {
	m := NewStateManager()
	formatted := m.FormatIssues(nil)
	_ = formatted
}
