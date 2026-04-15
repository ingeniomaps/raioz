package state

import (
	"testing"
	"time"
)

func TestLoadLocalState_Empty(t *testing.T) {
	dir := t.TempDir()
	state, err := LoadLocalState(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state == nil {
		t.Fatal("expected non-nil state")
	}
	if len(state.DevOverrides) != 0 {
		t.Error("expected empty dev overrides")
	}
}

func TestSaveAndLoadLocalState(t *testing.T) {
	dir := t.TempDir()
	state := &LocalState{
		Project:   "my-app",
		Workspace: "acme",
		LastUp:    time.Now().Truncate(time.Second),
		DevOverrides: map[string]DevOverride{
			"postgres": {
				OriginalImage: "postgres:16",
				LocalPath:     "/code/pg-local",
				PromotedAt:    time.Now().Truncate(time.Second),
			},
		},
		Ignored:     []string{"legacy-service"},
		HostPIDs:    map[string]int{"api": 12345},
		NetworkName: "acme-net",
	}

	err := SaveLocalState(dir, state)
	if err != nil {
		t.Fatalf("save failed: %v", err)
	}

	loaded, err := LoadLocalState(dir)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}

	if loaded.Project != "my-app" {
		t.Errorf("expected project 'my-app', got '%s'", loaded.Project)
	}
	if loaded.Workspace != "acme" {
		t.Errorf("expected workspace 'acme', got '%s'", loaded.Workspace)
	}

	override, ok := loaded.DevOverrides["postgres"]
	if !ok {
		t.Fatal("expected postgres dev override")
	}
	if override.OriginalImage != "postgres:16" {
		t.Errorf("expected original image 'postgres:16', got '%s'", override.OriginalImage)
	}
	if override.LocalPath != "/code/pg-local" {
		t.Errorf("expected local path '/code/pg-local', got '%s'", override.LocalPath)
	}

	if len(loaded.Ignored) != 1 || loaded.Ignored[0] != "legacy-service" {
		t.Errorf("expected ignored [legacy-service], got %v", loaded.Ignored)
	}
	if loaded.HostPIDs["api"] != 12345 {
		t.Errorf("expected api PID 12345, got %d", loaded.HostPIDs["api"])
	}
}

func TestDevOverrideMethods(t *testing.T) {
	state := &LocalState{
		DevOverrides: make(map[string]DevOverride),
	}

	if state.IsDevOverridden("postgres") {
		t.Error("should not be overridden initially")
	}

	state.AddDevOverride("postgres", "postgres:16", "/local/pg")

	if !state.IsDevOverridden("postgres") {
		t.Error("should be overridden after add")
	}

	override, ok := state.GetDevOverride("postgres")
	if !ok {
		t.Fatal("expected override to exist")
	}
	if override.OriginalImage != "postgres:16" {
		t.Errorf("expected 'postgres:16', got '%s'", override.OriginalImage)
	}

	state.RemoveDevOverride("postgres")

	if state.IsDevOverridden("postgres") {
		t.Error("should not be overridden after remove")
	}
}

func TestRemoveLocalState(t *testing.T) {
	dir := t.TempDir()
	state := &LocalState{Project: "test"}
	SaveLocalState(dir, state)

	err := RemoveLocalState(dir)
	if err != nil {
		t.Fatalf("remove failed: %v", err)
	}

	// Loading after remove should return empty state
	loaded, err := LoadLocalState(dir)
	if err != nil {
		t.Fatalf("load after remove failed: %v", err)
	}
	if loaded.Project != "" {
		t.Error("expected empty project after remove")
	}
}
