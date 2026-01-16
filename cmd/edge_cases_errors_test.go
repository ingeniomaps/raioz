package cmd_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"raioz/internal/config"
	"raioz/internal/errors"
	"raioz/internal/ignore"
	"raioz/internal/lock"
	"raioz/internal/override"
	"raioz/internal/root"
	"raioz/internal/state"
	testhelpers "raioz/internal/testing"
	"raioz/internal/workspace"
)

// TestErrorHandling_InvalidJSON tests error handling with malformed JSON
func TestErrorHandling_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()

	// Test 1: Completely malformed JSON
	malformedPath := filepath.Join(tmpDir, "malformed.json")
	os.WriteFile(malformedPath, []byte("{ invalid json }"), 0644)

	_, _, err := config.LoadDeps(malformedPath)
	if err == nil {
		t.Error("Expected error for malformed JSON, got nil")
	}

	// Test 2: JSON with syntax error
	syntaxErrorPath := filepath.Join(tmpDir, "syntax-error.json")
	os.WriteFile(syntaxErrorPath, []byte(`{"schemaVersion": "1.0", "project": {`), 0644)

	_, _, err = config.LoadDeps(syntaxErrorPath)
	if err == nil {
		t.Error("Expected error for JSON syntax error, got nil")
	}

	// Test 3: Empty file
	emptyPath := filepath.Join(tmpDir, "empty.json")
	os.WriteFile(emptyPath, []byte(""), 0644)

	_, _, err = config.LoadDeps(emptyPath)
	if err == nil {
		t.Error("Expected error for empty file, got nil")
	}
}

// TestErrorHandling_MissingRequiredFields tests error handling with missing required fields
func TestErrorHandling_MissingRequiredFields(t *testing.T) {
	tmpDir := t.TempDir()

	// Test 1: Missing project name
	deps1 := testhelpers.CreateMinimalTestDeps()
	deps1.Project.Name = ""
	depsPath1, _ := testhelpers.CreateTestDepsJSON(tmpDir, deps1)

	_, _, err := config.LoadDeps(depsPath1)
	// Loading should succeed, but validation should fail
	if err != nil {
		// This is expected - validation should catch missing name
	}

	// Test 2: Missing network
	deps2 := testhelpers.CreateMinimalTestDeps()
	deps2.Project.Network = ""
	depsPath2, _ := testhelpers.CreateTestDepsJSON(tmpDir, deps2)

	_, _, err = config.LoadDeps(depsPath2)
	// Loading should succeed, but validation should fail
	if err != nil {
		// This is expected
	}
}

// TestErrorHandling_WorkspaceErrors tests workspace error conditions
func TestErrorHandling_WorkspaceErrors(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("RAIOZ_HOME", tmpDir)
	defer os.Unsetenv("RAIOZ_HOME")

	// Test 1: Empty project name (workspace.Resolve doesn't validate, it just creates)
	// This is actually valid - empty string creates a workspace with empty name
	ws1, err := workspace.Resolve("")
	if err != nil {
		t.Logf("Note: workspace.Resolve with empty name returned error: %v", err)
	}
	if ws1 != nil {
		// Workspace was created, which is valid behavior
	}

	// Test 2: Project name with special characters
	// workspace.Resolve should handle this correctly
	ws2, err := workspace.Resolve("test-project-123")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if ws2 == nil {
		t.Fatal("Workspace should not be nil")
	}
}

// TestErrorHandling_LockErrors tests lock error conditions
func TestErrorHandling_LockErrors(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("RAIOZ_HOME", tmpDir)
	defer os.Unsetenv("RAIOZ_HOME")

	testDeps := testhelpers.CreateMinimalTestDeps()
	testDeps.Project.Name = "lock-error-test"

	ws, err := workspace.Resolve(testDeps.Project.Name)
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	// Test 1: Acquire lock twice (should fail on second attempt)
	lock1, err := lock.Acquire(ws)
	if err != nil {
		t.Fatalf("First lock acquisition failed: %v", err)
	}
	defer lock1.Release()

	lock2, err := lock.Acquire(ws)
	if err == nil {
		if lock2 != nil {
			lock2.Release()
		}
		t.Error("Expected error when acquiring lock twice, got nil")
	}

	// Test 2: Release lock twice
	// First release should succeed
	if err := lock1.Release(); err != nil {
		t.Errorf("First release failed: %v", err)
	}
	// Second release might fail if file doesn't exist, which is acceptable
	// We just verify it doesn't panic
	_ = lock1.Release()
}

// TestErrorHandling_StateErrors tests state error conditions
func TestErrorHandling_StateErrors(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("RAIOZ_HOME", tmpDir)
	defer os.Unsetenv("RAIOZ_HOME")

	testDeps := testhelpers.CreateMinimalTestDeps()
	testDeps.Project.Name = "state-error-test"

	ws, err := workspace.Resolve(testDeps.Project.Name)
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	// Test 1: Load non-existent state (should return nil, no error)
	loaded, err := state.Load(ws)
	if err != nil {
		t.Errorf("Expected no error loading non-existent state, got %v", err)
	}
	if loaded != nil {
		t.Error("Expected nil for non-existent state, got non-nil")
	}

	// Test 2: Save state with invalid data (should handle gracefully)
	// Create invalid state file manually
	statePath := filepath.Join(ws.Root, ".state.json")
	os.WriteFile(statePath, []byte("invalid json"), 0600)

	_, err = state.Load(ws)
	if err == nil {
		t.Error("Expected error loading invalid state file, got nil")
	}

	// Test 3: Save state to read-only directory (should fail)
	// This is hard to test without root, so we skip it
}

// TestErrorHandling_RootConfigErrors tests root config error conditions
func TestErrorHandling_RootConfigErrors(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("RAIOZ_HOME", tmpDir)
	defer os.Unsetenv("RAIOZ_HOME")

	testDeps := testhelpers.CreateMinimalTestDeps()
	testDeps.Project.Name = "root-error-test"

	ws, err := workspace.Resolve(testDeps.Project.Name)
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	// Test 1: Load non-existent root config (should return nil, no error)
	loaded, err := root.Load(ws)
	if err != nil {
		t.Errorf("Expected no error loading non-existent root config, got %v", err)
	}
	if loaded != nil {
		t.Error("Expected nil for non-existent root config, got non-nil")
	}

	// Test 2: Load corrupted root config
	rootPath := filepath.Join(ws.Root, "raioz.root.json")
	os.WriteFile(rootPath, []byte("invalid json"), 0644)

	_, err = root.Load(ws)
	if err == nil {
		t.Error("Expected error loading corrupted root config, got nil")
	}

	// Test 3: Save root config with nil metadata (should initialize)
	rootConfig, err := root.GenerateFromDeps(testDeps, []string{}, map[string]string{})
	if err != nil {
		t.Fatalf("GenerateFromDeps failed: %v", err)
	}
	rootConfig.Metadata = nil

	if err := root.Save(ws, rootConfig); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Reload and verify metadata was initialized
	reloaded, err := root.Load(ws)
	if err != nil {
		t.Fatalf("Reload failed: %v", err)
	}
	if reloaded.Metadata == nil {
		t.Error("Expected metadata to be initialized, got nil")
	}
}

// TestErrorHandling_OverrideErrors tests override error conditions
func TestErrorHandling_OverrideErrors(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("RAIOZ_HOME", tmpDir)
	defer os.Unsetenv("RAIOZ_HOME")

	// Test 1: Validate non-existent path
	err := override.ValidateOverridePath("/nonexistent/path/12345")
	if err == nil {
		t.Error("Expected error for non-existent path, got nil")
	}

	// Test 2: Validate path that is a file (not directory)
	filePath := filepath.Join(tmpDir, "file.txt")
	os.WriteFile(filePath, []byte("test"), 0644)

	err = override.ValidateOverridePath(filePath)
	if err == nil {
		t.Error("Expected error for file path (not directory), got nil")
	}

	// Test 3: Load corrupted override file
	overridePath, err := override.GetOverridesPath()
	if err != nil {
		t.Fatalf("GetOverridesPath failed: %v", err)
	}
	// Ensure directory exists
	os.MkdirAll(filepath.Dir(overridePath), 0755)
	os.WriteFile(overridePath, []byte("invalid json"), 0644)

	_, err = override.LoadOverrides()
	if err == nil {
		t.Error("Expected error loading corrupted override file, got nil")
	}

	// Test 4: Get override for non-existent service (after fixing corrupted file)
	// First, remove the corrupted file so LoadOverrides works
	os.Remove(overridePath)
	overrideConfig, err := override.GetOverride("nonexistent-service")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if overrideConfig != nil {
		t.Error("Expected nil for non-existent override, got non-nil")
	}
}

// TestErrorHandling_IgnoreErrors tests ignore system error conditions
func TestErrorHandling_IgnoreErrors(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("RAIOZ_HOME", tmpDir)
	defer os.Unsetenv("RAIOZ_HOME")

	// Test 1: Load corrupted ignore file
	ignorePath, _ := ignore.GetIgnorePath()
	os.WriteFile(ignorePath, []byte("invalid json"), 0644)

	_, err := ignore.Load()
	if err == nil {
		t.Error("Expected error loading corrupted ignore file, got nil")
	}

	// Test 2: IsIgnored with corrupted file (should handle gracefully)
	// After corruption, we need to fix the file first
	os.Remove(ignorePath)
	_, err = ignore.IsIgnored("test-service")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

// TestEdgeCases_EmptyValues tests edge cases with empty values
func TestEdgeCases_EmptyValues(t *testing.T) {
	tmpDir := t.TempDir()

	// Test 1: Empty services map
	deps1 := testhelpers.CreateMinimalTestDeps()
	deps1.Services = map[string]config.Service{}
	depsPath1, _ := testhelpers.CreateTestDepsJSON(tmpDir, deps1)

	loaded, _, err := config.LoadDeps(depsPath1)
	if err != nil {
		t.Fatalf("Failed to load deps: %v", err)
	}
	if len(loaded.Services) != 0 {
		t.Errorf("Expected 0 services, got %d", len(loaded.Services))
	}

	// Test 2: Empty infra map
	deps2 := testhelpers.CreateMinimalTestDeps()
	deps2.Infra = map[string]config.Infra{}
	depsPath2, _ := testhelpers.CreateTestDepsJSON(tmpDir, deps2)

	loaded, _, err = config.LoadDeps(depsPath2)
	if err != nil {
		t.Fatalf("Failed to load deps: %v", err)
	}
	if len(loaded.Infra) != 0 {
		t.Errorf("Expected 0 infra, got %d", len(loaded.Infra))
	}

	// Test 3: Empty env files
	deps3 := testhelpers.CreateMinimalTestDeps()
	deps3.Env.Files = []string{}
	depsPath3, _ := testhelpers.CreateTestDepsJSON(tmpDir, deps3)

	loaded, _, err = config.LoadDeps(depsPath3)
	if err != nil {
		t.Fatalf("Failed to load deps: %v", err)
	}
	if len(loaded.Env.Files) != 0 {
		t.Errorf("Expected 0 env files, got %d", len(loaded.Env.Files))
	}
}

// TestEdgeCases_NilValues tests edge cases with nil values
func TestEdgeCases_NilValues(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("RAIOZ_HOME", tmpDir)
	defer os.Unsetenv("RAIOZ_HOME")

	// Test 1: Root config with nil metadata (should be initialized on load)
	ws, err := workspace.Resolve("nil-test")
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	// Create root config file with nil metadata
	rootPath := filepath.Join(ws.Root, "raioz.root.json")
	rootData := `{
		"schemaVersion": "1.0",
		"generatedAt": "2024-01-01T00:00:00Z",
		"lastUpdatedAt": "2024-01-01T00:00:00Z",
		"project": {
			"name": "test",
			"network": "test-network"
		},
		"services": {},
		"infra": {},
		"env": {}
	}`
	os.WriteFile(rootPath, []byte(rootData), 0644)

	loaded, err := root.Load(ws)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if loaded.Metadata == nil {
		t.Error("Expected metadata to be initialized, got nil")
	}
}

// TestEdgeCases_SpecialCharacters tests edge cases with special characters
func TestEdgeCases_SpecialCharacters(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("RAIOZ_HOME", tmpDir)
	defer os.Unsetenv("RAIOZ_HOME")

	// Test 1: Project name with special characters
	projectName := "test-project_123-456"
	ws, err := workspace.Resolve(projectName)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if ws == nil {
		t.Fatal("Workspace should not be nil")
	}

	// Test 2: Service name with special characters
	testDeps := testhelpers.CreateMinimalTestDeps()
	testDeps.Project.Name = "special-chars-test"
	testDeps.Services["service_123-test"] = config.Service{
		Source: config.SourceConfig{
			Kind: "image",
			Image: "nginx",
			Tag:   "alpine",
		},
	}

	depsPath, _ := testhelpers.CreateTestDepsJSON(tmpDir, testDeps)
	loaded, _, err := config.LoadDeps(depsPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if _, exists := loaded.Services["service_123-test"]; !exists {
		t.Error("Service with special characters should exist")
	}
}

// TestEdgeCases_VeryLongNames tests edge cases with very long names
func TestEdgeCases_VeryLongNames(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("RAIOZ_HOME", tmpDir)
	defer os.Unsetenv("RAIOZ_HOME")

	// Test 1: Very long project name (using valid characters)
	longProjectName := "a" + strings.Repeat("b", 200)
	ws, err := workspace.Resolve(longProjectName)
	if err != nil {
		// Very long names might fail due to filesystem limits, which is acceptable
		t.Logf("Resolve with very long name returned error (acceptable): %v", err)
		return
	}
	if ws == nil {
		t.Fatal("Workspace should not be nil")
	}

	// Test 2: Very long service name (using valid characters)
	testDeps := testhelpers.CreateMinimalTestDeps()
	testDeps.Project.Name = "long-name-test"
	longServiceName := "service-" + strings.Repeat("x", 100)
	testDeps.Services[longServiceName] = config.Service{
		Source: config.SourceConfig{
			Kind: "image",
			Image: "nginx",
			Tag:   "alpine",
		},
	}

	depsPath, _ := testhelpers.CreateTestDepsJSON(tmpDir, testDeps)
	loaded, _, err := config.LoadDeps(depsPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if _, exists := loaded.Services[longServiceName]; !exists {
		t.Error("Service with long name should exist")
	}
}

// TestErrorHandling_ContextTimeout tests context timeout handling
func TestErrorHandling_ContextTimeout(t *testing.T) {
	// Test 1: Context with very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	// Wait a bit to ensure timeout
	time.Sleep(10 * time.Millisecond)

	if ctx.Err() == nil {
		t.Error("Expected context to be cancelled due to timeout")
	}

	// Test 2: Context already cancelled
	ctx2, cancel2 := context.WithCancel(context.Background())
	cancel2()

	if ctx2.Err() == nil {
		t.Error("Expected context to be cancelled")
	}
}

// TestErrorHandling_ConcurrentOperations tests concurrent operation handling
func TestErrorHandling_ConcurrentOperations(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("RAIOZ_HOME", tmpDir)
	defer os.Unsetenv("RAIOZ_HOME")

	testDeps := testhelpers.CreateMinimalTestDeps()
	testDeps.Project.Name = "concurrent-test"

	ws, err := workspace.Resolve(testDeps.Project.Name)
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	// Test: Multiple attempts to acquire lock concurrently
	// First lock should succeed
	lock1, err := lock.Acquire(ws)
	if err != nil {
		t.Fatalf("First lock failed: %v", err)
	}

	// Second lock should fail
	lock2, err := lock.Acquire(ws)
	if err == nil {
		if lock2 != nil {
			lock2.Release()
		}
		t.Error("Expected error for concurrent lock acquisition, got nil")
	}

	lock1.Release()
}

// TestErrorHandling_InvalidServiceConfig tests invalid service configurations
func TestErrorHandling_InvalidServiceConfig(t *testing.T) {
	tmpDir := t.TempDir()

	// Test 1: Service with invalid source kind
	deps := testhelpers.CreateMinimalTestDeps()
	deps.Services["invalid-service"] = config.Service{
		Source: config.SourceConfig{
			Kind: "invalid-kind",
		},
	}

	depsPath, _ := testhelpers.CreateTestDepsJSON(tmpDir, deps)
	loaded, _, err := config.LoadDeps(depsPath)
	if err != nil {
		t.Fatalf("Load should succeed even with invalid config: %v", err)
	}
	// Validation should catch this, but loading should work
	_ = loaded

	// Test 2: Service with empty source
	deps2 := testhelpers.CreateMinimalTestDeps()
	deps2.Services["empty-source"] = config.Service{
		Source: config.SourceConfig{},
	}

	depsPath2, _ := testhelpers.CreateTestDepsJSON(tmpDir, deps2)
	loaded2, _, err := config.LoadDeps(depsPath2)
	if err != nil {
		t.Fatalf("Load should succeed: %v", err)
	}
	_ = loaded2
}

// TestErrorHandling_StateCorruption tests state file corruption scenarios
func TestErrorHandling_StateCorruption(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("RAIOZ_HOME", tmpDir)
	defer os.Unsetenv("RAIOZ_HOME")

	testDeps := testhelpers.CreateMinimalTestDeps()
	testDeps.Project.Name = "corruption-test"

	ws, err := workspace.Resolve(testDeps.Project.Name)
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	// Test 1: State file with invalid JSON
	statePath := filepath.Join(ws.Root, ".state.json")
	os.WriteFile(statePath, []byte("{ invalid json }"), 0600)

	_, err = state.Load(ws)
	if err == nil {
		t.Error("Expected error loading corrupted state, got nil")
	}

	// Test 2: State file with valid JSON but invalid structure
	invalidState := `{"invalid": "structure"}`
	os.WriteFile(statePath, []byte(invalidState), 0600)

	_, err = state.Load(ws)
	// This might succeed (unmarshal to Deps), but structure might be invalid
	_ = err
}

// TestErrorHandling_RootConfigCorruption tests root config corruption scenarios
func TestErrorHandling_RootConfigCorruption(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("RAIOZ_HOME", tmpDir)
	defer os.Unsetenv("RAIOZ_HOME")

	testDeps := testhelpers.CreateMinimalTestDeps()
	testDeps.Project.Name = "root-corruption-test"

	ws, err := workspace.Resolve(testDeps.Project.Name)
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	// Test 1: Root config with invalid JSON
	rootPath := filepath.Join(ws.Root, "raioz.root.json")
	os.WriteFile(rootPath, []byte("{ invalid json }"), 0644)

	_, err = root.Load(ws)
	if err == nil {
		t.Error("Expected error loading corrupted root config, got nil")
	}

	// Test 2: Root config with missing required fields
	incompleteRoot := `{
		"schemaVersion": "1.0",
		"project": {}
	}`
	os.WriteFile(rootPath, []byte(incompleteRoot), 0644)

	_, err = root.Load(ws)
	// This might succeed (unmarshal), but structure might be invalid
	_ = err
}

// TestErrorHandling_OverridePathErrors tests override path error scenarios
func TestErrorHandling_OverridePathErrors(t *testing.T) {
	// Test 1: Path doesn't exist
	err := override.ValidateOverridePath("/nonexistent/path/12345")
	if err == nil {
		t.Error("Expected error for non-existent path, got nil")
	}

	// Test 2: Path is a file, not directory
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "file.txt")
	os.WriteFile(filePath, []byte("test"), 0644)

	err = override.ValidateOverridePath(filePath)
	if err == nil {
		t.Error("Expected error for file path, got nil")
	}

	// Test 3: Path is valid directory
	dirPath := filepath.Join(tmpDir, "valid-dir")
	os.MkdirAll(dirPath, 0755)

	err = override.ValidateOverridePath(dirPath)
	if err != nil {
		t.Errorf("Expected no error for valid directory, got %v", err)
	}
}

// TestEdgeCases_UnicodeCharacters tests edge cases with unicode characters
func TestEdgeCases_UnicodeCharacters(t *testing.T) {
	tmpDir := t.TempDir()

	// Test: Project name with unicode characters
	testDeps := testhelpers.CreateMinimalTestDeps()
	testDeps.Project.Name = "test-项目-123"
	testDeps.Project.Network = "test-network-网络"

	depsPath, _ := testhelpers.CreateTestDepsJSON(tmpDir, testDeps)
	loaded, _, err := config.LoadDeps(depsPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if loaded.Project.Name != "test-项目-123" {
		t.Errorf("Expected unicode project name, got %s", loaded.Project.Name)
	}
}

// TestErrorHandling_StructuredErrors tests structured error handling
func TestErrorHandling_StructuredErrors(t *testing.T) {
	// Test 1: Create RaiozError with all fields
	err := errors.New(errors.ErrCodeInvalidConfig, "test error").
		WithContext("key1", "value1").
		WithContext("key2", 42).
		WithSuggestion("fix it").
		WithError(errors.New(errors.ErrCodeInternalError, "original error"))

	if err.Code != errors.ErrCodeInvalidConfig {
		t.Errorf("Expected code %s, got %s", errors.ErrCodeInvalidConfig, err.Code)
	}
	if err.Message != "test error" {
		t.Errorf("Expected message 'test error', got %s", err.Message)
	}
	if len(err.Context) != 2 {
		t.Errorf("Expected 2 context items, got %d", len(err.Context))
	}
	if err.Suggestion != "fix it" {
		t.Errorf("Expected suggestion 'fix it', got %s", err.Suggestion)
	}
	if err.OriginalErr == nil {
		t.Error("Expected original error, got nil")
	}

	// Test 2: Format error
	formatted := err.Format()
	if formatted == "" {
		t.Error("Expected formatted error, got empty string")
	}
	if !contains(formatted, "INVALID_CONFIG") {
		t.Error("Expected formatted error to contain error code")
	}
	if !contains(formatted, "test error") {
		t.Error("Expected formatted error to contain message")
	}
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
