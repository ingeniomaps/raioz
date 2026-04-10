package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"raioz/internal/config"
	"raioz/internal/detect"
	"raioz/internal/state"
)

// TestLifecycle_InitDetectsEnvFiles verifies that init auto-detects .env.{dep} files
// and includes them in the generated config.
func TestLifecycle_InitDetectsEnvFiles(t *testing.T) {
	dir := t.TempDir()

	// Create a Go service with .env referencing postgres
	apiDir := filepath.Join(dir, "api")
	os.MkdirAll(apiDir, 0755)
	os.WriteFile(filepath.Join(apiDir, "go.mod"), []byte("module example.com/api\ngo 1.22\n"), 0644)
	os.WriteFile(filepath.Join(apiDir, ".env"), []byte("DATABASE_URL=postgres://localhost:5432/db\n"), 0644)

	// Create .env.postgres in project root
	os.WriteFile(filepath.Join(dir, ".env.postgres"), []byte("POSTGRES_PASSWORD=secret\n"), 0644)

	// Run auto-detect (same logic as raioz init / raioz up --auto)
	deps, err := config.AutoDetect(dir)
	if err != nil {
		t.Fatalf("AutoDetect failed: %v", err)
	}

	// Verify postgres dependency exists
	pg, ok := deps.Infra["postgres"]
	if !ok {
		t.Fatal("expected postgres dependency to be inferred")
	}
	if pg.Inline == nil {
		t.Fatal("expected inline infra config")
	}

	// Verify env file was detected
	if pg.Inline.Env == nil {
		t.Fatal("expected env to be set on postgres (should auto-detect .env.postgres)")
	}
	files := pg.Inline.Env.GetFilePaths()
	if len(files) == 0 || files[0] != ".env.postgres" {
		t.Errorf("expected env file '.env.postgres', got %v", files)
	}
}

// TestLifecycle_InitNoEnvFile verifies that init doesn't add env when no .env.{dep} exists.
func TestLifecycle_InitNoEnvFile(t *testing.T) {
	dir := t.TempDir()

	apiDir := filepath.Join(dir, "api")
	os.MkdirAll(apiDir, 0755)
	os.WriteFile(filepath.Join(apiDir, "go.mod"), []byte("module example.com/api\ngo 1.22\n"), 0644)
	os.WriteFile(filepath.Join(apiDir, ".env"), []byte("REDIS_URL=redis://localhost:6379\n"), 0644)

	// No .env.redis file created

	deps, err := config.AutoDetect(dir)
	if err != nil {
		t.Fatalf("AutoDetect failed: %v", err)
	}

	redis, ok := deps.Infra["redis"]
	if !ok {
		t.Fatal("expected redis dependency to be inferred")
	}
	if redis.Inline.Env != nil {
		t.Error("expected no env file for redis (no .env.redis exists)")
	}
}

// TestLifecycle_PIDPersistence verifies that host PIDs survive across state save/load.
func TestLifecycle_PIDPersistence(t *testing.T) {
	dir := t.TempDir()

	// Save state with PIDs
	st := &state.LocalState{
		Project:  "test-project",
		HostPIDs: map[string]int{"api": 12345, "web": 67890},
	}
	if err := state.SaveLocalState(dir, st); err != nil {
		t.Fatalf("SaveLocalState failed: %v", err)
	}

	// Load and verify
	loaded, err := state.LoadLocalState(dir)
	if err != nil {
		t.Fatalf("LoadLocalState failed: %v", err)
	}

	if loaded.HostPIDs["api"] != 12345 {
		t.Errorf("expected api PID 12345, got %d", loaded.HostPIDs["api"])
	}
	if loaded.HostPIDs["web"] != 67890 {
		t.Errorf("expected web PID 67890, got %d", loaded.HostPIDs["web"])
	}
}

// TestLifecycle_WatchConfigPreservedThroughBridge verifies watch: true survives
// YAML → Deps conversion and is accessible for the watcher.
func TestLifecycle_WatchConfigPreservedThroughBridge(t *testing.T) {
	dir := t.TempDir()

	// Create service directory
	apiDir := filepath.Join(dir, "api")
	os.MkdirAll(apiDir, 0755)
	os.WriteFile(filepath.Join(apiDir, "go.mod"), []byte("module example.com/api\ngo 1.22\n"), 0644)

	// Create raioz.yaml with watch: true
	yamlContent := `
project: test
services:
  api:
    path: ./api
    watch: true
  frontend:
    path: ./web
    watch: native
`
	yamlPath := filepath.Join(dir, "raioz.yaml")
	os.WriteFile(yamlPath, []byte(yamlContent), 0644)

	// Load through bridge
	deps, _, err := config.LoadDepsFromYAML(yamlPath)
	if err != nil {
		t.Fatalf("LoadDepsFromYAML failed: %v", err)
	}

	// Verify watch config
	api, ok := deps.Services["api"]
	if !ok {
		t.Fatal("expected api service")
	}
	if !api.Watch.Enabled {
		t.Error("expected api watch to be enabled")
	}
	if api.Watch.Mode != "" {
		t.Errorf("expected api watch mode empty, got %q", api.Watch.Mode)
	}

	frontend, ok := deps.Services["frontend"]
	if !ok {
		t.Fatal("expected frontend service")
	}
	if !frontend.Watch.Enabled {
		t.Error("expected frontend watch to be enabled")
	}
	if frontend.Watch.Mode != "native" {
		t.Errorf("expected frontend watch mode 'native', got %q", frontend.Watch.Mode)
	}
}

// TestLifecycle_ProxyConfigPreservedThroughFilters verifies proxy: true survives
// config loading → filtering → orchestration.
func TestLifecycle_ProxyConfigPreservedThroughFilters(t *testing.T) {
	dir := t.TempDir()

	yamlContent := `
project: test
proxy: true
services:
  api:
    path: ./api
dependencies:
  redis:
    image: redis:7
`
	yamlPath := filepath.Join(dir, "raioz.yaml")
	os.WriteFile(yamlPath, []byte(yamlContent), 0644)

	deps, _, err := config.LoadDepsFromYAML(yamlPath)
	if err != nil {
		t.Fatalf("LoadDepsFromYAML failed: %v", err)
	}

	if !deps.Proxy {
		t.Fatal("expected Proxy=true after loading YAML")
	}

	// Filter by feature flags (should preserve Proxy)
	filtered, _ := config.FilterByFeatureFlags(deps, "", map[string]string{})
	if !filtered.Proxy {
		t.Fatal("expected Proxy=true after FilterByFeatureFlags")
	}

	// Filter by profile (should preserve Proxy)
	profileFiltered := config.FilterByProfile(filtered, "")
	if !profileFiltered.Proxy {
		t.Fatal("expected Proxy=true after FilterByProfile")
	}
}

// TestLifecycle_PortInference verifies that host service ports are inferred
// from .env files and runtime defaults.
func TestLifecycle_PortInference(t *testing.T) {
	dir := t.TempDir()

	// Go service with PORT in .env
	apiDir := filepath.Join(dir, "api")
	os.MkdirAll(apiDir, 0755)
	os.WriteFile(filepath.Join(apiDir, "go.mod"), []byte("module example.com/api\ngo 1.22\n"), 0644)
	os.WriteFile(filepath.Join(apiDir, ".env"), []byte("PORT=9090\n"), 0644)

	result := detect.Detect(apiDir)
	if result.Runtime != detect.RuntimeGo {
		t.Fatalf("expected Go runtime, got %s", result.Runtime)
	}

	// Verify .env file has PORT
	data, _ := os.ReadFile(filepath.Join(apiDir, ".env"))
	if !strings.Contains(string(data), "PORT=9090") {
		t.Fatal("expected PORT=9090 in .env")
	}
}
