package e2e

import (
	"os"
	"path/filepath"
	"testing"

	"raioz/internal/config"
	"raioz/internal/detect"
	"raioz/internal/state"
)

// TestFullFlow_StatePersistence verifies the local state lifecycle:
// save → load → verify → add dev override → save → load → verify → remove → clean
func TestFullFlow_StatePersistence(t *testing.T) {
	dir := t.TempDir()

	// Save initial state
	s := &state.LocalState{
		Project:      "my-app",
		Workspace:    "acme",
		NetworkName:  "acme-net",
		DevOverrides: make(map[string]state.DevOverride),
		HostPIDs:     map[string]int{"api": 12345, "frontend": 12346},
		Ignored:      []string{"legacy"},
	}

	if err := state.SaveLocalState(dir, s); err != nil {
		t.Fatalf("SaveLocalState: %v", err)
	}

	// Verify file exists
	statePath := filepath.Join(dir, ".raioz.state.json")
	if _, err := os.Stat(statePath); err != nil {
		t.Fatal("state file not created")
	}

	// Load and verify
	loaded, err := state.LoadLocalState(dir)
	if err != nil {
		t.Fatalf("LoadLocalState: %v", err)
	}
	if loaded.Project != "my-app" {
		t.Errorf("project = %q, want 'my-app'", loaded.Project)
	}
	if loaded.HostPIDs["api"] != 12345 {
		t.Errorf("api PID = %d, want 12345", loaded.HostPIDs["api"])
	}
	if len(loaded.Ignored) != 1 || loaded.Ignored[0] != "legacy" {
		t.Errorf("ignored = %v, want [legacy]", loaded.Ignored)
	}

	// Add dev override
	loaded.AddDevOverride("postgres", "postgres:16", "/local/pg")
	if err := state.SaveLocalState(dir, loaded); err != nil {
		t.Fatalf("SaveLocalState after override: %v", err)
	}

	// Load again and verify override persists
	loaded2, err := state.LoadLocalState(dir)
	if err != nil {
		t.Fatalf("LoadLocalState 2: %v", err)
	}
	if !loaded2.IsDevOverridden("postgres") {
		t.Error("postgres should be dev-overridden")
	}
	override, _ := loaded2.GetDevOverride("postgres")
	if override.OriginalImage != "postgres:16" {
		t.Errorf("original image = %q, want 'postgres:16'", override.OriginalImage)
	}
	if override.LocalPath != "/local/pg" {
		t.Errorf("local path = %q, want '/local/pg'", override.LocalPath)
	}

	// Remove override
	loaded2.RemoveDevOverride("postgres")
	if loaded2.IsDevOverridden("postgres") {
		t.Error("postgres should not be overridden after remove")
	}

	// Clean state
	if err := state.RemoveLocalState(dir); err != nil {
		t.Fatalf("RemoveLocalState: %v", err)
	}
	if _, err := os.Stat(statePath); !os.IsNotExist(err) {
		t.Error("state file should be deleted")
	}
}

// TestFullFlow_DevSwapScenario simulates the full dev swap flow:
// 1. Load config with postgres as image dependency
// 2. Detect the local postgres path
// 3. Record the swap in state
// 4. Verify state reflects the swap
// 5. Reset and verify state is clean
func TestFullFlow_DevSwapScenario(t *testing.T) {
	dir := t.TempDir()

	// Create a local postgres directory
	pgDir := filepath.Join(dir, "local-pg")
	os.MkdirAll(pgDir, 0755)
	os.WriteFile(filepath.Join(pgDir, "Dockerfile"), []byte("FROM postgres:16\n"), 0644)

	// Load a config with postgres as dependency
	yamlContent := `
project: test-app
services:
  api:
    path: ./api
    dependsOn: [postgres]
dependencies:
  postgres:
    image: postgres:16
    ports: ["5432"]
`
	apiDir := filepath.Join(dir, "api")
	os.MkdirAll(apiDir, 0755)
	os.WriteFile(filepath.Join(apiDir, "go.mod"), []byte("module api\ngo 1.22\n"), 0644)

	configPath := filepath.Join(dir, "raioz.yaml")
	os.WriteFile(configPath, []byte(yamlContent), 0644)

	cfg, err := config.LoadYAML(configPath)
	if err != nil {
		t.Fatalf("LoadYAML: %v", err)
	}

	deps, err := config.YAMLToDeps(cfg)
	if err != nil {
		t.Fatalf("YAMLToDeps: %v", err)
	}

	// Verify postgres is an infra dependency
	pgEntry, ok := deps.Infra["postgres"]
	if !ok {
		t.Fatal("postgres not found in infra")
	}
	if pgEntry.Inline.Image != "postgres" {
		t.Errorf("postgres image = %q, want 'postgres'", pgEntry.Inline.Image)
	}

	// Detect the local path
	pgDetect := detect.Detect(pgDir)
	if pgDetect.Runtime != detect.RuntimeDockerfile {
		t.Errorf("local pg runtime = %s, want dockerfile", pgDetect.Runtime)
	}

	// Record the swap in state
	localState := &state.LocalState{
		Project:      "test-app",
		DevOverrides: make(map[string]state.DevOverride),
		HostPIDs:     make(map[string]int),
	}
	localState.AddDevOverride("postgres", "postgres:16", pgDir)

	if err := state.SaveLocalState(dir, localState); err != nil {
		t.Fatalf("SaveLocalState: %v", err)
	}

	// Load state and verify
	loaded, _ := state.LoadLocalState(dir)
	if !loaded.IsDevOverridden("postgres") {
		t.Error("postgres should be in dev mode")
	}

	// Reset
	loaded.RemoveDevOverride("postgres")
	state.SaveLocalState(dir, loaded)

	loaded2, _ := state.LoadLocalState(dir)
	if loaded2.IsDevOverridden("postgres") {
		t.Error("postgres should not be in dev mode after reset")
	}
}

// TestFullFlow_OrchestratorDispatch verifies the dispatcher routes
// each runtime to the correct runner type.
func TestFullFlow_OrchestratorDispatch(t *testing.T) {
	// Test that detection results map correctly to runner types
	tests := []struct {
		name     string
		runtime  detect.Runtime
		isDocker bool
		isHost   bool
	}{
		{"compose", detect.RuntimeCompose, true, false},
		{"dockerfile", detect.RuntimeDockerfile, true, false},
		{"image", detect.RuntimeImage, true, false},
		{"npm", detect.RuntimeNPM, false, true},
		{"go", detect.RuntimeGo, false, true},
		{"make", detect.RuntimeMake, false, true},
		{"python", detect.RuntimePython, false, true},
		{"rust", detect.RuntimeRust, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detect.DetectResult{Runtime: tt.runtime}
			if result.IsDocker() != tt.isDocker {
				t.Errorf("IsDocker() = %v, want %v", result.IsDocker(), tt.isDocker)
			}
			if result.IsHost() != tt.isHost {
				t.Errorf("IsHost() = %v, want %v", result.IsHost(), tt.isHost)
			}
		})
	}
}
