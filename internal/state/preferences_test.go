package state

import (
	"path/filepath"
	"testing"

	"raioz/internal/workspace"
)

func testWorkspace(t *testing.T) *workspace.Workspace {
	t.Helper()
	dir := t.TempDir()
	return &workspace.Workspace{
		Root: dir,
	}
}

func TestGetServicePreferencesPath(t *testing.T) {
	ws := testWorkspace(t)
	path := GetServicePreferencesPath(ws)
	if path == "" {
		t.Error("expected non-empty path")
	}
}

func TestGetServicePreferencesPath_Nil(t *testing.T) {
	if path := GetServicePreferencesPath(nil); path != "" {
		t.Errorf("expected empty for nil ws, got %q", path)
	}
}

func TestLoadServicePreferences_Empty(t *testing.T) {
	ws := testWorkspace(t)

	prefs, err := LoadServicePreferences(ws)
	if err != nil {
		t.Fatalf("LoadServicePreferences: %v", err)
	}
	if prefs == nil {
		t.Fatal("expected non-nil prefs")
	}
	if len(prefs.Preferences) != 0 {
		t.Errorf("expected empty prefs, got %d", len(prefs.Preferences))
	}
}

func TestLoadServicePreferences_NilWorkspace(t *testing.T) {
	prefs, err := LoadServicePreferences(nil)
	if err != nil {
		t.Fatalf("should handle nil workspace: %v", err)
	}
	if prefs == nil {
		t.Error("expected non-nil prefs")
	}
}

func TestSetAndGetServicePreference(t *testing.T) {
	ws := testWorkspace(t)

	pref := ServicePreference{
		ServiceName: "api",
		Preference:  "local",
		ProjectPath: "/local/api",
		Reason:      "developer choice",
	}
	if err := SetServicePreference(ws, pref); err != nil {
		t.Fatalf("SetServicePreference: %v", err)
	}

	got, err := GetServicePreference(ws, "api")
	if err != nil {
		t.Fatalf("GetServicePreference: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil preference")
	}
	if got.Preference != "local" {
		t.Errorf("Preference: got %q", got.Preference)
	}
	if got.ProjectPath != "/local/api" {
		t.Errorf("ProjectPath: got %q", got.ProjectPath)
	}
	if got.Timestamp.IsZero() {
		t.Error("Timestamp should be auto-set")
	}
}

func TestGetServicePreference_NotFound(t *testing.T) {
	ws := testWorkspace(t)

	got, err := GetServicePreference(ws, "nonexistent")
	if err != nil {
		t.Fatalf("GetServicePreference: %v", err)
	}
	if got != nil {
		t.Error("expected nil for non-existent preference")
	}
}

func TestRemoveServicePreference(t *testing.T) {
	ws := testWorkspace(t)

	// Add, then remove
	pref := ServicePreference{ServiceName: "api", Preference: "local"}
	_ = SetServicePreference(ws, pref)

	if err := RemoveServicePreference(ws, "api"); err != nil {
		t.Fatalf("RemoveServicePreference: %v", err)
	}

	got, _ := GetServicePreference(ws, "api")
	if got != nil {
		t.Error("preference should be removed")
	}
}

func TestSaveServicePreferences_NoWorkspaceRoot(t *testing.T) {
	ws := &workspace.Workspace{Root: ""}
	err := SaveServicePreferences(ws, &ServicePreferences{})
	if err == nil {
		t.Error("expected error for empty workspace root")
	}
}

func TestServicePreferences_Persists(t *testing.T) {
	dir := t.TempDir()
	ws := &workspace.Workspace{Root: dir}

	// Write prefs
	_ = SetServicePreference(ws, ServicePreference{
		ServiceName: "api",
		Preference:  "cloned",
		Workspace:   "ws1",
	})

	// Re-create ws with same root and load
	ws2 := &workspace.Workspace{Root: dir}
	prefs, err := LoadServicePreferences(ws2)
	if err != nil {
		t.Fatalf("LoadServicePreferences: %v", err)
	}
	if _, ok := prefs.Preferences["api"]; !ok {
		t.Error("preference not persisted")
	}
}

func TestGetServicePreferencesPath_Structure(t *testing.T) {
	dir := t.TempDir()
	ws := &workspace.Workspace{Root: dir}
	got := GetServicePreferencesPath(ws)
	want := filepath.Join(dir, "service-preferences.json")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// Workspace preferences tests

func TestGetWorkspaceProjectPreference_Empty(t *testing.T) {
	setupGlobalHome(t)

	pref, err := GetWorkspaceProjectPreference("nonexistent")
	if err != nil {
		t.Fatalf("GetWorkspaceProjectPreference: %v", err)
	}
	// Non-existent preference should return nil (or empty pref)
	_ = pref
}

func TestSetAndGetWorkspaceProjectPreference(t *testing.T) {
	setupGlobalHome(t)

	pref := WorkspaceProjectPreference{
		PreferredProject: "proj1",
		AlwaysAsk:        false,
	}
	if err := SetWorkspaceProjectPreference("ws1", pref); err != nil {
		t.Fatalf("SetWorkspaceProjectPreference: %v", err)
	}

	got, err := GetWorkspaceProjectPreference("ws1")
	if err != nil {
		t.Fatalf("GetWorkspaceProjectPreference: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil preference")
	}
	if got.PreferredProject != "proj1" {
		t.Errorf("got %q, want proj1", got.PreferredProject)
	}
}
