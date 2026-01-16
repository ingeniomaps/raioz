package env

import (
	"os"
	"path/filepath"
	"testing"

	"raioz/internal/config"
	"raioz/internal/workspace"
)

func TestResolveEnvFiles(t *testing.T) {
	tmpDir := t.TempDir()
	ws := &workspace.Workspace{
		Root:        filepath.Join(tmpDir, "workspace"),
		ServicesDir: filepath.Join(tmpDir, "services"),
		EnvDir:      filepath.Join(tmpDir, "env"),
	}

	// Create env directory structure
	if err := EnsureEnvDirs(ws); err != nil {
		t.Fatalf("Failed to create env dirs: %v", err)
	}

	// Create test env files
	globalEnv := "GLOBAL_VAR=global_value\n"
	if err := os.WriteFile(filepath.Join(ws.EnvDir, "global.env"), []byte(globalEnv), 0644); err != nil {
		t.Fatalf("Failed to create global.env: %v", err)
	}

	projectEnv := "PROJECT_VAR=project_value\n"
	projectPath := filepath.Join(ws.EnvDir, "projects", "test-project.env")
	if err := os.WriteFile(projectPath, []byte(projectEnv), 0644); err != nil {
		t.Fatalf("Failed to create project.env: %v", err)
	}

	serviceEnv := "SERVICE_VAR=service_value\n"
	servicePath := filepath.Join(ws.EnvDir, "services", "test-service.env")
	if err := os.WriteFile(servicePath, []byte(serviceEnv), 0644); err != nil {
		t.Fatalf("Failed to create service.env: %v", err)
	}

	deps := &config.Deps{
		Env: config.EnvConfig{
			UseGlobal: true,
			Files:     []string{"projects/test-project"},
		},
	}

	tests := []struct {
		name      string
		envFiles  []string
		wantCount int
	}{
		{
			name:      "with global, project, and service",
			envFiles:  []string{"services/test-service"},
			wantCount: 3, // global + project + service
		},
		{
			name:      "with global and project only",
			envFiles:  []string{},
			wantCount: 2, // global + project
		},
		{
			name:      "without global",
			envFiles:  []string{"services/test-service"},
			wantCount: 2, // project + service (no global)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "without global" {
				deps.Env.UseGlobal = false
			} else {
				deps.Env.UseGlobal = true
			}

			paths, err := ResolveEnvFiles(ws, deps, "test-service", tt.envFiles, "")
			if err != nil {
				t.Errorf("ResolveEnvFiles() error = %v", err)
				return
			}

			if len(paths) != tt.wantCount {
				t.Errorf("ResolveEnvFiles() got %d paths, want %d", len(paths), tt.wantCount)
			}
		})
	}
}

func TestLoadFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test env files
	file1 := filepath.Join(tmpDir, "file1.env")
	file2 := filepath.Join(tmpDir, "file2.env")

	if err := os.WriteFile(file1, []byte("VAR1=value1\nVAR2=value2\n"), 0644); err != nil {
		t.Fatalf("Failed to create file1.env: %v", err)
	}

	if err := os.WriteFile(file2, []byte("VAR2=value2_override\nVAR3=value3\n"), 0644); err != nil {
		t.Fatalf("Failed to create file2.env: %v", err)
	}

	env, err := LoadFiles([]string{file1, file2})
	if err != nil {
		t.Fatalf("LoadFiles() error = %v", err)
	}

	if env["VAR1"] != "value1" {
		t.Errorf("LoadFiles() VAR1 = %v, want value1", env["VAR1"])
	}

	if env["VAR2"] != "value2_override" {
		t.Errorf("LoadFiles() VAR2 = %v, want value2_override (later file should override)", env["VAR2"])
	}

	if env["VAR3"] != "value3" {
		t.Errorf("LoadFiles() VAR3 = %v, want value3", env["VAR3"])
	}
}

func TestLoadSingleFile(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.env")

	content := `# This is a comment
VAR1=value1
VAR2=value with spaces
VAR3="quoted value"
VAR4='single quoted'
# Another comment

VAR5=normal_value
`

	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test.env: %v", err)
	}

	env, err := loadSingleFile(filePath)
	if err != nil {
		t.Fatalf("loadSingleFile() error = %v", err)
	}

	if env["VAR1"] != "value1" {
		t.Errorf("loadSingleFile() VAR1 = %v, want value1", env["VAR1"])
	}

	if env["VAR2"] != "value with spaces" {
		t.Errorf("loadSingleFile() VAR2 = %v, want 'value with spaces'", env["VAR2"])
	}

	if env["VAR3"] != "quoted value" {
		t.Errorf("loadSingleFile() VAR3 = %v, want 'quoted value'", env["VAR3"])
	}

	if env["VAR4"] != "single quoted" {
		t.Errorf("loadSingleFile() VAR4 = %v, want 'single quoted'", env["VAR4"])
	}

	if env["VAR5"] != "normal_value" {
		t.Errorf("loadSingleFile() VAR5 = %v, want normal_value", env["VAR5"])
	}

	// Should not have comment lines
	if _, exists := env["# This is a comment"]; exists {
		t.Error("loadSingleFile() should not include comment lines")
	}
}

func TestResolveEnvFileForService(t *testing.T) {
	tmpDir := t.TempDir()
	ws := &workspace.Workspace{
		Root:        filepath.Join(tmpDir, "workspace"),
		ServicesDir: filepath.Join(tmpDir, "services"),
		EnvDir:      filepath.Join(tmpDir, "env"),
	}

	if err := EnsureEnvDirs(ws); err != nil {
		t.Fatalf("Failed to create env dirs: %v", err)
	}

	// Create env files
	globalContent := "GLOBAL=global_value\n"
	if err := os.WriteFile(filepath.Join(ws.EnvDir, "global.env"), []byte(globalContent), 0644); err != nil {
		t.Fatalf("Failed to create global.env: %v", err)
	}

	serviceContent := "SERVICE=service_value\n"
	servicePath := filepath.Join(ws.EnvDir, "services", "my-service.env")
	if err := os.WriteFile(servicePath, []byte(serviceContent), 0644); err != nil {
		t.Fatalf("Failed to create service.env: %v", err)
	}

	deps := &config.Deps{
		Env: config.EnvConfig{
			UseGlobal: true,
			Files:     []string{},
		},
	}

	// Test with single file (global only)
	envPath, err := ResolveEnvFileForService(ws, deps, "my-service", []string{})
	if err != nil {
		t.Fatalf("ResolveEnvFileForService() error = %v", err)
	}

	if envPath == "" {
		t.Error("ResolveEnvFileForService() should return a path for global env")
	}

	// Verify the combined file contains both values
	env, err := LoadFiles([]string{envPath})
	if err != nil {
		t.Fatalf("Failed to load combined env file: %v", err)
	}

	if env["GLOBAL"] != "global_value" {
		t.Errorf("Combined env GLOBAL = %v, want global_value", env["GLOBAL"])
	}

	// Test with service file
	envPath2, err := ResolveEnvFileForService(ws, deps, "my-service", []string{"services/my-service"})
	if err != nil {
		t.Fatalf("ResolveEnvFileForService() error = %v", err)
	}

	env2, err := LoadFiles([]string{envPath2})
	if err != nil {
		t.Fatalf("Failed to load combined env file: %v", err)
	}

	if env2["GLOBAL"] != "global_value" {
		t.Errorf("Combined env GLOBAL = %v, want global_value", env2["GLOBAL"])
	}

	if env2["SERVICE"] != "service_value" {
		t.Errorf("Combined env SERVICE = %v, want service_value", env2["SERVICE"])
	}
}

func TestEnsureEnvDirs(t *testing.T) {
	tmpDir := t.TempDir()
	ws := &workspace.Workspace{
		Root:        filepath.Join(tmpDir, "workspace"),
		ServicesDir: filepath.Join(tmpDir, "services"),
		EnvDir:      filepath.Join(tmpDir, "env"),
	}

	if err := EnsureEnvDirs(ws); err != nil {
		t.Fatalf("EnsureEnvDirs() error = %v", err)
	}

	// Verify directories exist
	dirs := []string{
		ws.EnvDir,
		filepath.Join(ws.EnvDir, "services"),
		filepath.Join(ws.EnvDir, "projects"),
	}

	for _, dir := range dirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			t.Errorf("EnsureEnvDirs() directory %s was not created", dir)
		}
	}
}
