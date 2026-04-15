package docker

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCleanUnusedVolumesWithContext_NotDryRunNotForce(t *testing.T) {
	// When dryRun=false and force=false, should return error
	_, err := CleanUnusedVolumesWithContext(
		context.Background(), false, false,
	)
	if err == nil {
		t.Error("expected error when neither dryRun nor force")
	}
	if !strings.Contains(err.Error(), "--force required") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCleanUnusedImages_Wrapper(t *testing.T) {
	requireDocker(t)
	// Exercise the wrapper (calls WithContext)
	actions, err := CleanUnusedImages(true) // dry run
	if err != nil {
		t.Fatalf("dry run err: %v", err)
	}
	if len(actions) == 0 {
		t.Error("expected at least one action")
	}
}

func TestCleanUnusedImagesWithContext_DryRun(t *testing.T) {
	requireDocker(t)
	actions, err := CleanUnusedImagesWithContext(
		context.Background(), true,
	)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(actions) == 0 {
		t.Error("expected at least one action for dry run")
	}
}

func TestCleanUnusedNetworks_Wrapper(t *testing.T) {
	requireDocker(t)
	actions, err := CleanUnusedNetworks(true)
	if err != nil {
		t.Fatalf("dry run err: %v", err)
	}
	if len(actions) == 0 {
		t.Error("expected at least one action")
	}
}

func TestCleanUnusedNetworksWithContext_DryRun(t *testing.T) {
	requireDocker(t)
	actions, err := CleanUnusedNetworksWithContext(
		context.Background(), true,
	)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(actions) == 0 {
		t.Error("expected at least one action for dry run")
	}
}

func TestCleanProjectWithContext_DryRunValidFile(t *testing.T) {
	tmp := t.TempDir()
	compose := filepath.Join(tmp, "docker-compose.yml")
	if err := os.WriteFile(compose, []byte("services: {}"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	actions, err := CleanProjectWithContext(
		context.Background(), compose, true,
	)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d: %v", len(actions), actions)
	}
	if !strings.Contains(actions[0], "Would remove") {
		t.Errorf("unexpected action: %q", actions[0])
	}
}

func TestCleanProjectWithContext_MissingFile(t *testing.T) {
	tmp := t.TempDir()
	compose := filepath.Join(tmp, "nonexistent.yml")

	// non-existent file: returns empty actions, no error
	actions, err := CleanProjectWithContext(
		context.Background(), compose, false,
	)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(actions) != 0 {
		t.Errorf("expected 0 actions, got %v", actions)
	}
}

func TestCleanAllProjectsWithContext_WithStateFile(t *testing.T) {
	tmp := t.TempDir()
	projDir := filepath.Join(tmp, "workspaces", "proj1")
	if err := os.MkdirAll(projDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Only state file, no compose file
	statePath := filepath.Join(projDir, ".state.json")
	if err := os.WriteFile(statePath, []byte("{}"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Dry run: should report state file
	actions, err := CleanAllProjectsWithContext(
		context.Background(), tmp, true,
	)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	foundState := false
	for _, a := range actions {
		if strings.Contains(a, "state file") {
			foundState = true
		}
	}
	if !foundState {
		t.Errorf("expected state file action, got: %v", actions)
	}
}

func TestCleanAllProjectsWithContext_ActualCleanStateFile(t *testing.T) {
	tmp := t.TempDir()
	projDir := filepath.Join(tmp, "workspaces", "proj1")
	if err := os.MkdirAll(projDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	statePath := filepath.Join(projDir, ".state.json")
	if err := os.WriteFile(statePath, []byte("{}"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Non-dry run: should remove state file
	actions, err := CleanAllProjectsWithContext(
		context.Background(), tmp, false,
	)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Verify state file is removed
	if _, err := os.Stat(statePath); !os.IsNotExist(err) {
		t.Error("expected state file to be removed")
	}

	foundRemoved := false
	for _, a := range actions {
		if strings.Contains(a, "Removed state file") {
			foundRemoved = true
		}
	}
	if !foundRemoved {
		t.Errorf("expected removed action, got: %v", actions)
	}
}
