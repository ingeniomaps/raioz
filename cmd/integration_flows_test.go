package cmd_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"raioz/internal/config"
	"raioz/internal/ignore"
	"raioz/internal/lock"
	"raioz/internal/override"
	"raioz/internal/root"
	"raioz/internal/state"
	testhelpers "raioz/internal/testing"
	"raioz/internal/workspace"
)

// TestCompleteUpFlow tests the complete flow of raioz up
// This tests the integration of multiple components without actually running Docker
func TestCompleteUpFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir := t.TempDir()
	os.Setenv("RAIOZ_HOME", tmpDir)
	defer os.Unsetenv("RAIOZ_HOME")

	// Create test configuration
	testDeps := testhelpers.CreateMinimalTestDeps()
	testDeps.Project.Name = "integration-up-test"
	testDeps.Project.Network = "test-network"

	// Add a service with Git source
	testDeps.Services["api"] = config.Service{
		Source: config.SourceConfig{
			Kind: "image",
			Image: "nginx",
			Tag:   "alpine",
		},
		Docker: config.DockerConfig{
			Mode:  "dev",
			Ports: []string{"8080:80"},
		},
	}

	// Add infra
	testDeps.Infra["database"] = config.Infra{
		Image: "postgres",
		Tag:   "15-alpine",
		Ports: []string{"5432:5432"},
	}

	// Create .raioz.json
	depsPath := filepath.Join(tmpDir, ".raioz.json")
	depsData, err := json.MarshalIndent(testDeps, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal deps: %v", err)
	}
	if err := os.WriteFile(depsPath, depsData, 0644); err != nil {
		t.Fatalf("Failed to write .raioz.json: %v", err)
	}

	// Change to temp directory
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	os.Chdir(tmpDir)

	// Step 1: Load configuration
	loadedDeps, warnings, err := config.LoadDeps(depsPath)
	if err != nil {
		t.Fatalf("Step 1 - Load config failed: %v", err)
	}
	if len(warnings) > 0 {
		t.Logf("Warnings during load: %v", warnings)
	}
	if loadedDeps.Project.Name != testDeps.Project.Name {
		t.Errorf("Step 1 - Expected project name %s, got %s",
			testDeps.Project.Name, loadedDeps.Project.Name)
	}

	// Step 2: Resolve workspace
	ws, err := workspace.Resolve(loadedDeps.Project.Name)
	if err != nil {
		t.Fatalf("Step 2 - Resolve workspace failed: %v", err)
	}
	if ws == nil {
		t.Fatal("Step 2 - Workspace is nil")
	}
	if ws.Root == "" {
		t.Error("Step 2 - Workspace root is empty")
	}

	// Step 3: Acquire lock
	lock, err := lock.Acquire(ws)
	if err != nil {
		t.Fatalf("Step 3 - Acquire lock failed: %v", err)
	}
	defer lock.Release()

	// Step 4: Save state
	if err := state.Save(ws, loadedDeps); err != nil {
		t.Fatalf("Step 4 - Save state failed: %v", err)
	}

	// Step 5: Verify state was saved
	savedState, err := state.Load(ws)
	if err != nil {
		t.Fatalf("Step 5 - Load state failed: %v", err)
	}
	if savedState == nil {
		t.Fatal("Step 5 - Saved state is nil")
	}
	if savedState.Project.Name != loadedDeps.Project.Name {
		t.Errorf("Step 5 - Expected project name %s, got %s",
			loadedDeps.Project.Name, savedState.Project.Name)
	}

	// Step 6: Generate root config
	rootConfig, err := root.GenerateFromDeps(loadedDeps, []string{}, map[string]string{})
	if err != nil {
		t.Fatalf("Step 6 - Generate root config failed: %v", err)
	}
	if rootConfig == nil {
		t.Fatal("Step 6 - Root config is nil")
	}
	if rootConfig.Project.Name != loadedDeps.Project.Name {
		t.Errorf("Step 6 - Expected project name %s, got %s",
			loadedDeps.Project.Name, rootConfig.Project.Name)
	}

	// Step 7: Save root config
	if err := root.Save(ws, rootConfig); err != nil {
		t.Fatalf("Step 7 - Save root config failed: %v", err)
	}

	// Step 8: Verify root config was saved
	if !root.Exists(ws) {
		t.Error("Step 8 - Root config file does not exist")
	}
	loadedRoot, err := root.Load(ws)
	if err != nil {
		t.Fatalf("Step 8 - Load root config failed: %v", err)
	}
	if loadedRoot == nil {
		t.Fatal("Step 8 - Loaded root config is nil")
	}
	if loadedRoot.Project.Name != rootConfig.Project.Name {
		t.Errorf("Step 8 - Expected project name %s, got %s",
			rootConfig.Project.Name, loadedRoot.Project.Name)
	}

	t.Log("Complete up flow test passed")
}

// TestCompleteDownFlow tests the complete flow of raioz down
func TestCompleteDownFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir := t.TempDir()
	os.Setenv("RAIOZ_HOME", tmpDir)
	defer os.Unsetenv("RAIOZ_HOME")

	// Setup: Create workspace and state (simulating a running project)
	testDeps := testhelpers.CreateMinimalTestDeps()
	testDeps.Project.Name = "integration-down-test"

	ws, err := workspace.Resolve(testDeps.Project.Name)
	if err != nil {
		t.Fatalf("Setup - Resolve workspace failed: %v", err)
	}

	// Save state (simulating a project that was up)
	if err := state.Save(ws, testDeps); err != nil {
		t.Fatalf("Setup - Save state failed: %v", err)
	}

	// Save root config
	rootConfig, err := root.GenerateFromDeps(testDeps, []string{}, map[string]string{})
	if err != nil {
		t.Fatalf("Setup - Generate root config failed: %v", err)
	}
	if err := root.Save(ws, rootConfig); err != nil {
		t.Fatalf("Setup - Save root config failed: %v", err)
	}

	// Step 1: Verify state exists before down
	if !state.Exists(ws) {
		t.Error("Step 1 - State should exist before down")
	}
	if !root.Exists(ws) {
		t.Error("Step 1 - Root config should exist before down")
	}

	// Step 2: Load state (simulating what down would do)
	loadedState, err := state.Load(ws)
	if err != nil {
		t.Fatalf("Step 2 - Load state failed: %v", err)
	}
	if loadedState == nil {
		t.Fatal("Step 2 - Loaded state is nil")
	}

	// Step 3: Load root config
	loadedRoot, err := root.Load(ws)
	if err != nil {
		t.Fatalf("Step 3 - Load root config failed: %v", err)
	}
	if loadedRoot == nil {
		t.Fatal("Step 3 - Loaded root config is nil")
	}

	// Note: Actual Docker down would happen here, but we skip it in tests
	// The state and root config remain after down (they're not deleted)

	t.Log("Complete down flow test passed")
}

// TestWorkspaceManagementFlow tests workspace creation and management
func TestWorkspaceManagementFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir := t.TempDir()
	os.Setenv("RAIOZ_HOME", tmpDir)
	defer os.Unsetenv("RAIOZ_HOME")

	projectName := "workspace-test"

	// Step 1: Resolve workspace (creates it)
	ws, err := workspace.Resolve(projectName)
	if err != nil {
		t.Fatalf("Step 1 - Resolve workspace failed: %v", err)
	}

	// Step 2: Verify workspace structure
	if ws.Root == "" {
		t.Error("Step 2 - Workspace root is empty")
	}
	if ws.ServicesDir == "" {
		t.Error("Step 2 - Services directory is empty")
	}
	if ws.LocalServicesDir == "" {
		t.Error("Step 2 - Local services directory is empty")
	}
	if ws.ReadonlyServicesDir == "" {
		t.Error("Step 2 - Readonly services directory is empty")
	}
	if ws.EnvDir == "" {
		t.Error("Step 2 - Env directory is empty")
	}

	// Step 3: Verify directories exist
	if _, err := os.Stat(ws.Root); os.IsNotExist(err) {
		t.Errorf("Step 3 - Workspace root does not exist: %s", ws.Root)
	}
	if _, err := os.Stat(ws.LocalServicesDir); os.IsNotExist(err) {
		t.Errorf("Step 3 - Local services directory does not exist: %s", ws.LocalServicesDir)
	}
	if _, err := os.Stat(ws.ReadonlyServicesDir); os.IsNotExist(err) {
		t.Errorf("Step 3 - Readonly services directory does not exist: %s", ws.ReadonlyServicesDir)
	}

	// Step 4: Verify helper functions
	baseDir := workspace.GetBaseDirFromWorkspace(ws)
	if baseDir == "" {
		t.Error("Step 4 - Base directory is empty")
	}

	statePath := workspace.GetStatePath(ws)
	if statePath == "" {
		t.Error("Step 4 - State path is empty")
	}
	if statePath != filepath.Join(ws.Root, ".state.json") {
		t.Errorf("Step 4 - Expected state path %s, got %s",
			filepath.Join(ws.Root, ".state.json"), statePath)
	}

	composePath := workspace.GetComposePath(ws)
	if composePath == "" {
		t.Error("Step 4 - Compose path is empty")
	}

	t.Log("Workspace management flow test passed")
}

// TestOverrideSystemFlow tests the complete override system flow
func TestOverrideSystemFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir := t.TempDir()
	os.Setenv("RAIOZ_HOME", tmpDir)
	defer os.Unsetenv("RAIOZ_HOME")

	serviceName := "test-service"
	overridePath := filepath.Join(tmpDir, "external-service")

	// Create external directory
	if err := os.MkdirAll(overridePath, 0755); err != nil {
		t.Fatalf("Setup - Failed to create external directory: %v", err)
	}

	// Step 1: Add override
	overrideConfig := override.Override{
		Path:   overridePath,
		Mode:   "local",
		Source: "external",
	}
	if err := override.AddOverride(serviceName, overrideConfig); err != nil {
		t.Fatalf("Step 1 - Add override failed: %v", err)
	}

	// Step 2: Verify override exists
	hasOverride, err := override.HasOverride(serviceName)
	if err != nil {
		t.Fatalf("Step 2 - HasOverride failed: %v", err)
	}
	if !hasOverride {
		t.Error("Step 2 - Override should exist")
	}

	// Step 3: Get override
	loadedOverride, err := override.GetOverride(serviceName)
	if err != nil {
		t.Fatalf("Step 3 - GetOverride failed: %v", err)
	}
	if loadedOverride == nil {
		t.Fatal("Step 3 - Override is nil")
	}
	if loadedOverride.Path != overridePath {
		t.Errorf("Step 3 - Expected path %s, got %s", overridePath, loadedOverride.Path)
	}

	// Step 4: Validate override path
	if err := override.ValidateOverridePath(overridePath); err != nil {
		t.Fatalf("Step 4 - ValidateOverridePath failed: %v", err)
	}

	// Step 5: Load all overrides
	overrides, err := override.LoadOverrides()
	if err != nil {
		t.Fatalf("Step 5 - LoadOverrides failed: %v", err)
	}
	if len(overrides) != 1 {
		t.Errorf("Step 5 - Expected 1 override, got %d", len(overrides))
	}

	// Step 6: Remove override
	if err := override.RemoveOverride(serviceName); err != nil {
		t.Fatalf("Step 6 - RemoveOverride failed: %v", err)
	}

	// Step 7: Verify override was removed
	hasOverride, err = override.HasOverride(serviceName)
	if err != nil {
		t.Fatalf("Step 7 - HasOverride failed: %v", err)
	}
	if hasOverride {
		t.Error("Step 7 - Override should not exist")
	}

	t.Log("Override system flow test passed")
}

// TestIgnoreSystemFlow tests the complete ignore system flow
func TestIgnoreSystemFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir := t.TempDir()
	os.Setenv("RAIOZ_HOME", tmpDir)
	defer os.Unsetenv("RAIOZ_HOME")

	serviceName := "ignored-service"

	// Step 1: Add service to ignore list
	if err := ignore.AddService(serviceName); err != nil {
		t.Fatalf("Step 1 - AddService failed: %v", err)
	}

	// Step 2: Verify service is ignored
	isIgnored, err := ignore.IsIgnored(serviceName)
	if err != nil {
		t.Fatalf("Step 2 - IsIgnored failed: %v", err)
	}
	if !isIgnored {
		t.Error("Step 2 - Service should be ignored")
	}

	// Step 3: Get ignored services
	ignoredServices, err := ignore.GetIgnoredServices()
	if err != nil {
		t.Fatalf("Step 3 - GetIgnoredServices failed: %v", err)
	}
	if len(ignoredServices) != 1 {
		t.Errorf("Step 3 - Expected 1 ignored service, got %d", len(ignoredServices))
	}
	if ignoredServices[0] != serviceName {
		t.Errorf("Step 3 - Expected service %s, got %s", serviceName, ignoredServices[0])
	}

	// Step 4: Remove service from ignore list
	if err := ignore.RemoveService(serviceName); err != nil {
		t.Fatalf("Step 4 - RemoveService failed: %v", err)
	}

	// Step 5: Verify service is not ignored
	isIgnored, err = ignore.IsIgnored(serviceName)
	if err != nil {
		t.Fatalf("Step 5 - IsIgnored failed: %v", err)
	}
	if isIgnored {
		t.Error("Step 5 - Service should not be ignored")
	}

	t.Log("Ignore system flow test passed")
}

// TestRootConfigFlow tests the complete root config flow
func TestRootConfigFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir := t.TempDir()
	os.Setenv("RAIOZ_HOME", tmpDir)
	defer os.Unsetenv("RAIOZ_HOME")

	testDeps := testhelpers.CreateMinimalTestDeps()
	testDeps.Project.Name = "root-config-test"

	ws, err := workspace.Resolve(testDeps.Project.Name)
	if err != nil {
		t.Fatalf("Setup - Resolve workspace failed: %v", err)
	}

	// Step 1: Generate root config from deps
	rootConfig, err := root.GenerateFromDeps(testDeps, []string{}, map[string]string{})
	if err != nil {
		t.Fatalf("Step 1 - GenerateFromDeps failed: %v", err)
	}
	if rootConfig == nil {
		t.Fatal("Step 1 - Root config is nil")
	}

	// Step 2: Save root config
	if err := root.Save(ws, rootConfig); err != nil {
		t.Fatalf("Step 2 - Save root config failed: %v", err)
	}

	// Step 3: Verify root config exists
	if !root.Exists(ws) {
		t.Error("Step 3 - Root config should exist")
	}

	// Step 4: Load root config
	loadedRoot, err := root.Load(ws)
	if err != nil {
		t.Fatalf("Step 4 - Load root config failed: %v", err)
	}
	if loadedRoot == nil {
		t.Fatal("Step 4 - Loaded root config is nil")
	}

	// Step 5: Convert root config back to deps
	depsFromRoot := loadedRoot.ToDeps()
	if depsFromRoot.Project.Name != testDeps.Project.Name {
		t.Errorf("Step 5 - Expected project name %s, got %s",
			testDeps.Project.Name, depsFromRoot.Project.Name)
	}

	// Step 6: Update root config with new deps
	testDeps.Project.Network = "new-network"
	if err := root.UpdateFromDeps(loadedRoot, testDeps, []string{}, map[string]string{}); err != nil {
		t.Fatalf("Step 6 - UpdateFromDeps failed: %v", err)
	}

	// Step 7: Verify update
	if loadedRoot.Project.Network != "new-network" {
		t.Errorf("Step 7 - Expected network new-network, got %s", loadedRoot.Project.Network)
	}
	if loadedRoot.LastUpdatedAt == "" {
		t.Error("Step 7 - LastUpdatedAt should be set")
	}

	// Step 8: Add assisted service
	assistedService := config.Service{
		Source: config.SourceConfig{
			Kind: "image",
			Image: "nginx",
			Tag:   "alpine",
		},
	}
	loadedRoot.AddAssistedService("assisted-service", assistedService, "api", "required dependency")

	// Step 9: Verify assisted service was added
	if _, exists := loadedRoot.Services["assisted-service"]; !exists {
		t.Error("Step 9 - Assisted service should exist")
	}
	if loadedRoot.Metadata["assisted-service"].Origin != root.OriginAssisted {
		t.Errorf("Step 9 - Expected origin %s, got %s",
			root.OriginAssisted, loadedRoot.Metadata["assisted-service"].Origin)
	}

	t.Log("Root config flow test passed")
}

// TestStateManagementFlow tests state save and load flow
func TestStateManagementFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir := t.TempDir()
	os.Setenv("RAIOZ_HOME", tmpDir)
	defer os.Unsetenv("RAIOZ_HOME")

	testDeps := testhelpers.CreateMinimalTestDeps()
	testDeps.Project.Name = "state-test"

	ws, err := workspace.Resolve(testDeps.Project.Name)
	if err != nil {
		t.Fatalf("Setup - Resolve workspace failed: %v", err)
	}

	// Step 1: State should not exist initially
	if state.Exists(ws) {
		t.Error("Step 1 - State should not exist initially")
	}

	// Step 2: Save state
	if err := state.Save(ws, testDeps); err != nil {
		t.Fatalf("Step 2 - Save state failed: %v", err)
	}

	// Step 3: Verify state exists
	if !state.Exists(ws) {
		t.Error("Step 3 - State should exist after save")
	}

	// Step 4: Load state
	loadedState, err := state.Load(ws)
	if err != nil {
		t.Fatalf("Step 4 - Load state failed: %v", err)
	}
	if loadedState == nil {
		t.Fatal("Step 4 - Loaded state is nil")
	}

	// Step 5: Verify state content
	if loadedState.Project.Name != testDeps.Project.Name {
		t.Errorf("Step 5 - Expected project name %s, got %s",
			testDeps.Project.Name, loadedState.Project.Name)
	}

	// Step 6: Update and save again (idempotency)
	testDeps.Project.Network = "updated-network"
	if err := state.Save(ws, testDeps); err != nil {
		t.Fatalf("Step 6 - Save state again failed: %v", err)
	}

	// Step 7: Verify update
	reloadedState, err := state.Load(ws)
	if err != nil {
		t.Fatalf("Step 7 - Reload state failed: %v", err)
	}
	if reloadedState.Project.Network != "updated-network" {
		t.Errorf("Step 7 - Expected network updated-network, got %s",
			reloadedState.Project.Network)
	}

	t.Log("State management flow test passed")
}

// TestLockFlow tests the lock acquisition and release flow
func TestLockFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir := t.TempDir()
	os.Setenv("RAIOZ_HOME", tmpDir)
	defer os.Unsetenv("RAIOZ_HOME")

	testDeps := testhelpers.CreateMinimalTestDeps()
	testDeps.Project.Name = "lock-test"

	ws, err := workspace.Resolve(testDeps.Project.Name)
	if err != nil {
		t.Fatalf("Setup - Resolve workspace failed: %v", err)
	}

	// Step 1: Acquire first lock
	lock1, err := lock.Acquire(ws)
	if err != nil {
		t.Fatalf("Step 1 - Acquire first lock failed: %v", err)
	}

	// Step 2: Try to acquire second lock (should fail)
	lock2, err := lock.Acquire(ws)
	if err == nil {
		if lock2 != nil {
			lock2.Release()
		}
		t.Error("Step 2 - Acquiring second lock should fail")
	}

	// Step 3: Release first lock
	if err := lock1.Release(); err != nil {
		t.Fatalf("Step 3 - Release lock failed: %v", err)
	}

	// Step 4: Acquire lock again (should succeed)
	lock3, err := lock.Acquire(ws)
	if err != nil {
		t.Fatalf("Step 4 - Acquire lock after release failed: %v", err)
	}
	defer lock3.Release()

	t.Log("Lock flow test passed")
}

// TestContextPropagationFlow tests that context is properly propagated
func TestContextPropagationFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Step 1: Verify context is not cancelled
	if ctx.Err() != nil {
		t.Fatal("Step 1 - Context should not be cancelled")
	}

	// Step 2: Verify context has deadline
	deadline, ok := ctx.Deadline()
	if !ok {
		t.Error("Step 2 - Context should have deadline")
	}
	if deadline.IsZero() {
		t.Error("Step 2 - Deadline should not be zero")
	}

	// Step 3: Cancel context
	cancel()

	// Step 4: Verify context is cancelled
	if ctx.Err() == nil {
		t.Error("Step 4 - Context should be cancelled")
	}

	t.Log("Context propagation flow test passed")
}
