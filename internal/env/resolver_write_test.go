package env

import (
	"os"
	"path/filepath"
	"testing"

	"raioz/internal/config"
	"raioz/internal/workspace"
)

func makeTestWS(t *testing.T) *workspace.Workspace {
	t.Helper()
	dir := t.TempDir()
	return &workspace.Workspace{
		Root:        filepath.Join(dir, "ws"),
		ServicesDir: filepath.Join(dir, "services"),
		EnvDir:      filepath.Join(dir, "env"),
	}
}

func TestCreateOrUpdateEnvFile_NewFile(t *testing.T) {
	ws := makeTestWS(t)
	_ = EnsureEnvDirs(ws)

	deps := &config.Deps{
		Project: config.Project{Name: "myproj"},
	}

	vars := map[string]string{
		"DB_URL": "postgres://localhost",
		"PORT":   "5432",
	}

	path, err := CreateOrUpdateEnvFile(ws, deps, "api", vars, "")
	if err != nil {
		t.Fatalf("CreateOrUpdateEnvFile: %v", err)
	}
	if path == "" {
		t.Error("expected non-empty path")
	}

	// Read back and verify
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("readback: %v", err)
	}
	parsed := parseEnvContent(string(data))
	if parsed["DB_URL"] != "postgres://localhost" {
		t.Errorf("DB_URL: got %q", parsed["DB_URL"])
	}
	if parsed["PORT"] != "5432" {
		t.Errorf("PORT: got %q", parsed["PORT"])
	}
}

func TestCreateOrUpdateEnvFile_MergeExisting(t *testing.T) {
	ws := makeTestWS(t)
	_ = EnsureEnvDirs(ws)

	deps := &config.Deps{
		Project: config.Project{Name: "myproj"},
	}

	// First write
	_, err := CreateOrUpdateEnvFile(ws, deps, "api", map[string]string{
		"OLD":  "value",
		"KEEP": "keep",
	}, "")
	if err != nil {
		t.Fatalf("first write: %v", err)
	}

	// Second write merges
	path, err := CreateOrUpdateEnvFile(ws, deps, "api", map[string]string{
		"NEW":  "new_val",
		"KEEP": "updated",
	}, "")
	if err != nil {
		t.Fatalf("second write: %v", err)
	}

	data, _ := os.ReadFile(path)
	parsed := parseEnvContent(string(data))

	if parsed["OLD"] != "value" {
		t.Errorf("OLD should be preserved: %q", parsed["OLD"])
	}
	if parsed["KEEP"] != "updated" {
		t.Errorf("KEEP should be updated: %q", parsed["KEEP"])
	}
	if parsed["NEW"] != "new_val" {
		t.Errorf("NEW should be added: %q", parsed["NEW"])
	}
}

func TestCreateOrUpdateEnvFile_ServicePath(t *testing.T) {
	ws := makeTestWS(t)

	deps := &config.Deps{
		Project: config.Project{Name: "myproj"},
	}

	servicePath := t.TempDir()
	path, err := CreateOrUpdateEnvFile(
		ws, deps, "api",
		map[string]string{"X": "y"},
		servicePath,
	)
	if err != nil {
		t.Fatalf("CreateOrUpdateEnvFile: %v", err)
	}

	// Should be written to servicePath/.env
	expected := filepath.Join(servicePath, ".env")
	if path != expected {
		t.Errorf("got %q, want %q", path, expected)
	}
}

func TestWriteGlobalEnvVariables_Disabled(t *testing.T) {
	ws := makeTestWS(t)

	deps := &config.Deps{
		Env: config.EnvConfig{
			UseGlobal: false,
			Files:     []string{},
			Variables: map[string]string{},
		},
	}

	// Should skip silently
	err := WriteGlobalEnvVariables(ws, deps, "")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestWriteGlobalEnvVariables_WithVariables(t *testing.T) {
	ws := makeTestWS(t)
	projectDir := t.TempDir()

	deps := &config.Deps{
		Env: config.EnvConfig{
			UseGlobal: true,
			Variables: map[string]string{
				"FOO": "bar",
				"DB":  "postgres",
			},
		},
	}

	if err := WriteGlobalEnvVariables(ws, deps, projectDir); err != nil {
		t.Fatalf("WriteGlobalEnvVariables: %v", err)
	}

	globalPath := filepath.Join(ws.EnvDir, "global.env")
	data, err := os.ReadFile(globalPath)
	if err != nil {
		t.Fatalf("readback: %v", err)
	}

	parsed := parseEnvContent(string(data))
	if parsed["FOO"] != "bar" {
		t.Errorf("FOO: got %q", parsed["FOO"])
	}
	if parsed["DB"] != "postgres" {
		t.Errorf("DB: got %q", parsed["DB"])
	}
}

func TestWriteGlobalEnvVariables_WithFiles(t *testing.T) {
	ws := makeTestWS(t)
	projectDir := t.TempDir()

	// Create an env file in the project dir
	envFile := filepath.Join(projectDir, ".env.global")
	os.WriteFile(envFile, []byte("KEY1=val1\nKEY2=val2\n"), 0o644)

	deps := &config.Deps{
		Env: config.EnvConfig{
			UseGlobal: true,
			Files:     []string{".env.global"},
		},
	}

	if err := WriteGlobalEnvVariables(ws, deps, projectDir); err != nil {
		t.Fatalf("WriteGlobalEnvVariables: %v", err)
	}

	globalPath := filepath.Join(ws.EnvDir, "global.env")
	data, _ := os.ReadFile(globalPath)
	parsed := parseEnvContent(string(data))

	if parsed["KEY1"] != "val1" || parsed["KEY2"] != "val2" {
		t.Errorf("env file not merged: %v", parsed)
	}
}

func TestWriteGlobalEnvVariables_MergesExisting(t *testing.T) {
	ws := makeTestWS(t)
	projectDir := t.TempDir()

	_ = EnsureEnvDirs(ws)

	// Write initial global.env
	globalPath := filepath.Join(ws.EnvDir, "global.env")
	os.WriteFile(globalPath, []byte("OLD=old_val\nKEEP=keep_old\n"), 0o600)

	deps := &config.Deps{
		Env: config.EnvConfig{
			UseGlobal: true,
			Variables: map[string]string{
				"NEW":  "new_val",
				"KEEP": "updated",
			},
		},
	}

	if err := WriteGlobalEnvVariables(ws, deps, projectDir); err != nil {
		t.Fatalf("WriteGlobalEnvVariables: %v", err)
	}

	data, _ := os.ReadFile(globalPath)
	parsed := parseEnvContent(string(data))

	if parsed["OLD"] != "old_val" {
		t.Errorf("OLD should be preserved: %q", parsed["OLD"])
	}
	if parsed["KEEP"] != "updated" {
		t.Errorf("KEEP should be updated: %q", parsed["KEEP"])
	}
	if parsed["NEW"] != "new_val" {
		t.Errorf("NEW should be added: %q", parsed["NEW"])
	}
}

func TestEnsureEnvDirs_Creates(t *testing.T) {
	ws := makeTestWS(t)

	if err := EnsureEnvDirs(ws); err != nil {
		t.Fatalf("EnsureEnvDirs: %v", err)
	}

	dirs := []string{
		ws.EnvDir,
		filepath.Join(ws.EnvDir, "services"),
		filepath.Join(ws.EnvDir, "projects"),
	}
	for _, d := range dirs {
		if _, err := os.Stat(d); err != nil {
			t.Errorf("dir %q not created: %v", d, err)
		}
	}
}

func TestEnsureEnvDirs_Idempotent(t *testing.T) {
	ws := makeTestWS(t)

	// Call twice
	if err := EnsureEnvDirs(ws); err != nil {
		t.Fatalf("first call: %v", err)
	}
	if err := EnsureEnvDirs(ws); err != nil {
		t.Fatalf("second call: %v", err)
	}
}
