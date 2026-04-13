package env

import (
	"os"
	"path/filepath"
	"testing"

	"raioz/internal/config"
	"raioz/internal/domain/interfaces"
)

func setupWS(t *testing.T) (*interfaces.Workspace, string) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("RAIOZ_HOME", dir)

	wsDir := filepath.Join(dir, "testproj")
	envDir := filepath.Join(wsDir, "env")
	if err := os.MkdirAll(envDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	ws := &interfaces.Workspace{
		Root:                wsDir,
		ServicesDir:         filepath.Join(wsDir, "services"),
		LocalServicesDir:    filepath.Join(wsDir, "local"),
		ReadonlyServicesDir: filepath.Join(wsDir, "readonly"),
		EnvDir:              envDir,
	}
	return ws, wsDir
}

func TestNewEnvManager(t *testing.T) {
	m := NewEnvManager()
	if m == nil {
		t.Fatal("NewEnvManager returned nil")
	}
}

func TestEnvManagerImpl_ResolveProjectEnv(t *testing.T) {
	m := NewEnvManager()
	ws, projectDir := setupWS(t)

	deps := &config.Deps{
		Project: config.Project{Name: "testproj"},
	}

	// With no env configured — should not panic
	_, _ = m.ResolveProjectEnv(ws, deps, projectDir)
}

func TestEnvManagerImpl_GenerateEnvFromTemplate_NoTemplate(t *testing.T) {
	m := NewEnvManager()
	ws, projectDir := setupWS(t)

	deps := &config.Deps{
		Project: config.Project{Name: "testproj"},
	}
	svc := config.Service{}

	servicePath := filepath.Join(projectDir, "api")
	if err := os.MkdirAll(servicePath, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// No template — should not error
	err := m.GenerateEnvFromTemplate(
		ws, deps, "api", servicePath, svc, "", projectDir,
	)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestEnvManagerImpl_WriteGlobalEnvVariables(t *testing.T) {
	m := NewEnvManager()
	ws, projectDir := setupWS(t)

	deps := &config.Deps{
		Project: config.Project{Name: "testproj"},
	}

	// May or may not produce an env file depending on config.
	// Just verify it doesn't crash.
	_ = m.WriteGlobalEnvVariables(ws, deps, projectDir)
}

func TestEnvManagerImpl_ResolveEnvFiles(t *testing.T) {
	m := NewEnvManager()
	ws, projectDir := setupWS(t)

	deps := &config.Deps{
		Project: config.Project{Name: "testproj"},
	}

	// Create an env file
	envFile := filepath.Join(projectDir, ".env.api")
	if err := os.WriteFile(envFile, []byte("FOO=bar"), 0o644); err != nil {
		t.Fatalf("write env: %v", err)
	}

	files, err := m.ResolveEnvFiles(
		ws, deps, "api", []string{envFile}, "", false, projectDir,
	)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	_ = files
}
