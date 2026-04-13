package workspace

import (
	"os"
	"path/filepath"
	"testing"

	"raioz/internal/config"
)

func TestMigrateLegacyServices(t *testing.T) {
	// Create temporary directories
	tmpDir := t.TempDir()
	baseDir := filepath.Join(tmpDir, "base")
	workspaceRoot := filepath.Join(baseDir, "workspaces", "test-project")
	oldServicesDir := filepath.Join(baseDir, "services")
	localServicesDir := filepath.Join(workspaceRoot, "local")
	readonlyServicesDir := filepath.Join(workspaceRoot, "readonly")

	// Create directories
	if err := os.MkdirAll(oldServicesDir, 0755); err != nil {
		t.Fatalf("Failed to create old services dir: %v", err)
	}
	if err := os.MkdirAll(localServicesDir, 0755); err != nil {
		t.Fatalf("Failed to create local services dir: %v", err)
	}
	if err := os.MkdirAll(readonlyServicesDir, 0755); err != nil {
		t.Fatalf("Failed to create readonly services dir: %v", err)
	}

	// Create a legacy service in old location
	legacyServicePath := filepath.Join(oldServicesDir, "services", "legacy-service")
	if err := os.MkdirAll(legacyServicePath, 0755); err != nil {
		t.Fatalf("Failed to create legacy service dir: %v", err)
	}
	// Create a .git directory to simulate a git repo
	gitDir := filepath.Join(legacyServicePath, ".git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatalf("Failed to create .git dir: %v", err)
	}

	ws := &Workspace{
		Root:                workspaceRoot,
		ServicesDir:         oldServicesDir,
		LocalServicesDir:    localServicesDir,
		ReadonlyServicesDir: readonlyServicesDir,
		EnvDir:              filepath.Join(baseDir, "env"),
	}

	deps := &config.Deps{
		SchemaVersion: "1.0",
		Project: config.Project{
			Name: "test-project",
		},
		Services: map[string]config.Service{
			"legacy-service": {
				Source: config.SourceConfig{
					Kind:   "git",
					Repo:   "git@github.com:org/legacy.git",
					Branch: "main",
					Path:   "services/legacy-service",
					Access: "editable", // Should go to local/
				},
			},
		},
		Infra: map[string]config.InfraEntry{},
	}

	// Perform migration
	err := MigrateLegacyServices(ws, deps)
	if err != nil {
		t.Fatalf("Migration failed: %v", err)
	}

	// Verify service was moved to local/
	expectedNewPath := filepath.Join(localServicesDir, "services", "legacy-service")
	if _, err := os.Stat(expectedNewPath); os.IsNotExist(err) {
		t.Errorf("Service was not migrated to %s", expectedNewPath)
	}

	// Verify old location no longer exists
	if _, err := os.Stat(legacyServicePath); err == nil {
		t.Errorf("Old service path still exists: %s", legacyServicePath)
	}
}

func TestMigrateLegacyServicesReadonly(t *testing.T) {
	// Create temporary directories
	tmpDir := t.TempDir()
	baseDir := filepath.Join(tmpDir, "base")
	workspaceRoot := filepath.Join(baseDir, "workspaces", "test-project")
	oldServicesDir := filepath.Join(baseDir, "services")
	localServicesDir := filepath.Join(workspaceRoot, "local")
	readonlyServicesDir := filepath.Join(workspaceRoot, "readonly")

	// Create directories
	if err := os.MkdirAll(oldServicesDir, 0755); err != nil {
		t.Fatalf("Failed to create old services dir: %v", err)
	}
	if err := os.MkdirAll(localServicesDir, 0755); err != nil {
		t.Fatalf("Failed to create local services dir: %v", err)
	}
	if err := os.MkdirAll(readonlyServicesDir, 0755); err != nil {
		t.Fatalf("Failed to create readonly services dir: %v", err)
	}

	// Create a legacy readonly service in old location
	legacyServicePath := filepath.Join(oldServicesDir, "services", "readonly-service")
	if err := os.MkdirAll(legacyServicePath, 0755); err != nil {
		t.Fatalf("Failed to create legacy service dir: %v", err)
	}
	// Create a .git directory to simulate a git repo
	gitDir := filepath.Join(legacyServicePath, ".git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatalf("Failed to create .git dir: %v", err)
	}

	ws := &Workspace{
		Root:                workspaceRoot,
		ServicesDir:         oldServicesDir,
		LocalServicesDir:    localServicesDir,
		ReadonlyServicesDir: readonlyServicesDir,
		EnvDir:              filepath.Join(baseDir, "env"),
	}

	deps := &config.Deps{
		SchemaVersion: "1.0",
		Project: config.Project{
			Name: "test-project",
		},
		Services: map[string]config.Service{
			"readonly-service": {
				Source: config.SourceConfig{
					Kind:   "git",
					Repo:   "git@github.com:org/readonly.git",
					Branch: "main",
					Path:   "services/readonly-service",
					Access: "readonly", // Should go to readonly/
				},
			},
		},
		Infra: map[string]config.InfraEntry{},
	}

	// Perform migration
	err := MigrateLegacyServices(ws, deps)
	if err != nil {
		t.Fatalf("Migration failed: %v", err)
	}

	// Verify service was moved to readonly/
	expectedNewPath := filepath.Join(readonlyServicesDir, "services", "readonly-service")
	if _, err := os.Stat(expectedNewPath); os.IsNotExist(err) {
		t.Errorf("Service was not migrated to %s", expectedNewPath)
	}

	// Verify old location no longer exists
	if _, err := os.Stat(legacyServicePath); err == nil {
		t.Errorf("Old service path still exists: %s", legacyServicePath)
	}
}

func TestMigrateLegacyServicesSkipIfExists(t *testing.T) {
	// Create temporary directories
	tmpDir := t.TempDir()
	baseDir := filepath.Join(tmpDir, "base")
	workspaceRoot := filepath.Join(baseDir, "workspaces", "test-project")
	oldServicesDir := filepath.Join(baseDir, "services")
	localServicesDir := filepath.Join(workspaceRoot, "local")
	readonlyServicesDir := filepath.Join(workspaceRoot, "readonly")

	// Create directories
	if err := os.MkdirAll(oldServicesDir, 0755); err != nil {
		t.Fatalf("Failed to create old services dir: %v", err)
	}
	if err := os.MkdirAll(localServicesDir, 0755); err != nil {
		t.Fatalf("Failed to create local services dir: %v", err)
	}

	// Create service in both old and new locations
	legacyServicePath := filepath.Join(oldServicesDir, "services", "existing-service")
	newServicePath := filepath.Join(localServicesDir, "services", "existing-service")
	if err := os.MkdirAll(legacyServicePath, 0755); err != nil {
		t.Fatalf("Failed to create legacy service dir: %v", err)
	}
	if err := os.MkdirAll(newServicePath, 0755); err != nil {
		t.Fatalf("Failed to create new service dir: %v", err)
	}

	ws := &Workspace{
		Root:                workspaceRoot,
		ServicesDir:         oldServicesDir,
		LocalServicesDir:    localServicesDir,
		ReadonlyServicesDir: readonlyServicesDir,
		EnvDir:              filepath.Join(baseDir, "env"),
	}

	deps := &config.Deps{
		SchemaVersion: "1.0",
		Project: config.Project{
			Name: "test-project",
		},
		Services: map[string]config.Service{
			"existing-service": {
				Source: config.SourceConfig{
					Kind:   "git",
					Repo:   "git@github.com:org/existing.git",
					Branch: "main",
					Path:   "services/existing-service",
					Access: "editable",
				},
			},
		},
		Infra: map[string]config.InfraEntry{},
	}

	// Perform migration - should skip since service already exists in new location
	err := MigrateLegacyServices(ws, deps)
	if err != nil {
		t.Fatalf("Migration failed: %v", err)
	}

	// Verify old location still exists (should not be moved)
	if _, err := os.Stat(legacyServicePath); os.IsNotExist(err) {
		t.Errorf("Old service path was moved but should have been skipped: %s", legacyServicePath)
	}

	// Verify new location still exists
	if _, err := os.Stat(newServicePath); os.IsNotExist(err) {
		t.Errorf("New service path does not exist: %s", newServicePath)
	}
}
