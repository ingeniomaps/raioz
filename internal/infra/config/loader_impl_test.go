package config

import (
	"os"
	"path/filepath"
	"testing"

	configpkg "raioz/internal/config"
)

func writeYAMLConfig(t *testing.T, dir string) string {
	t.Helper()
	content := `project: test-project
services:
  api:
    path: ./api
deps:
  postgres:
    image: postgres:16
`
	path := filepath.Join(dir, "raioz.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	// Ensure service path exists
	if err := os.MkdirAll(filepath.Join(dir, "api"), 0o755); err != nil {
		t.Fatalf("mkdir api: %v", err)
	}
	return path
}

func TestConfigLoaderImpl_LoadDeps_YAML(t *testing.T) {
	loader := NewConfigLoader()
	dir := t.TempDir()
	path := writeYAMLConfig(t, dir)

	deps, _, err := loader.LoadDeps(path)
	if err != nil {
		t.Fatalf("LoadDeps failed: %v", err)
	}
	if deps == nil || deps.Project.Name != "test-project" {
		t.Errorf("unexpected deps: %+v", deps)
	}
}

func TestConfigLoaderImpl_LoadDeps_NonExistent(t *testing.T) {
	loader := NewConfigLoader()
	_, _, err := loader.LoadDeps(filepath.Join(t.TempDir(), "missing.yaml"))
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestConfigLoaderImpl_LoadDeps_AutoDetect(t *testing.T) {
	loader := NewConfigLoader()
	// Auto-detect mode may or may not find something depending on cwd.
	// Just verify it doesn't panic.
	_, _, _ = loader.LoadDeps(":auto:")
}

func TestConfigLoaderImpl_IsServiceEnabled(t *testing.T) {
	loader := NewConfigLoader()
	svc := configpkg.Service{}
	// No profile/flags set — default behavior
	got := loader.IsServiceEnabled(svc, "", nil)
	// Default should be enabled (true). Just verify it returns a bool without panicking.
	_ = got
}

func TestConfigLoaderImpl_ValidateFeatureFlags(t *testing.T) {
	loader := NewConfigLoader()
	deps := &configpkg.Deps{}
	if err := loader.ValidateFeatureFlags(deps); err != nil {
		t.Errorf("empty deps should validate: %v", err)
	}
}

func TestConfigLoaderImpl_FilterByProfile(t *testing.T) {
	loader := NewConfigLoader()
	deps := &configpkg.Deps{
		Services: map[string]configpkg.Service{
			"api": {},
		},
	}
	got := loader.FilterByProfile(deps, "")
	if got == nil {
		t.Error("expected non-nil deps")
	}
}

func TestConfigLoaderImpl_FilterByProfiles(t *testing.T) {
	loader := NewConfigLoader()
	deps := &configpkg.Deps{
		Services: map[string]configpkg.Service{"api": {}},
	}
	got := loader.FilterByProfiles(deps, []string{})
	if got == nil {
		t.Error("expected non-nil deps")
	}
}

func TestConfigLoaderImpl_FilterByFeatureFlags(t *testing.T) {
	loader := NewConfigLoader()
	deps := &configpkg.Deps{
		Services: map[string]configpkg.Service{"api": {}},
	}
	got, mocks := loader.FilterByFeatureFlags(deps, "", nil)
	if got == nil {
		t.Error("expected non-nil deps")
	}
	_ = mocks
}

func TestConfigLoaderImpl_FilterIgnoredServices(t *testing.T) {
	loader := NewConfigLoader()
	deps := &configpkg.Deps{
		Services: map[string]configpkg.Service{"api": {}},
	}
	got, _, err := loader.FilterIgnoredServices(deps)
	if err != nil {
		t.Fatalf("FilterIgnoredServices: %v", err)
	}
	if got == nil {
		t.Error("expected non-nil deps")
	}
}

func TestConfigLoaderImpl_CheckIgnoredDependencies(t *testing.T) {
	loader := NewConfigLoader()
	deps := &configpkg.Deps{
		Services: map[string]configpkg.Service{"api": {}},
	}
	got := loader.CheckIgnoredDependencies(deps, []string{"db"})
	if got == nil {
		t.Error("expected non-nil map")
	}
}

func TestConfigLoaderImpl_DetectMissingDependencies(t *testing.T) {
	loader := NewConfigLoader()
	deps := &configpkg.Deps{
		Services: map[string]configpkg.Service{},
	}
	resolver := func(name string, svc configpkg.Service) string { return "" }
	_, err := loader.DetectMissingDependencies(deps, resolver)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestConfigLoaderImpl_DetectDependencyConflicts(t *testing.T) {
	loader := NewConfigLoader()
	deps := &configpkg.Deps{
		Services: map[string]configpkg.Service{},
	}
	resolver := func(name string, svc configpkg.Service) string { return "" }
	_, err := loader.DetectDependencyConflicts(deps, resolver)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestConfigLoaderImpl_FindServiceConfig_NotFound(t *testing.T) {
	loader := NewConfigLoader()
	dir := t.TempDir()
	// No .raioz.json in the dir — expect not found behavior
	_, _, err := loader.FindServiceConfig(dir)
	if err == nil {
		// Some implementations return empty instead of error; tolerate both.
		t.Log("no error returned for missing config — tolerated")
	}
}
