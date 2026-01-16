package workspace

import (
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"testing"
)

func TestGetBaseDir(t *testing.T) {
	// Save original RAIOZ_HOME
	originalHome := os.Getenv("RAIOZ_HOME")
	defer os.Setenv("RAIOZ_HOME", originalHome)

	// Test 1: With RAIOZ_HOME set
	testDir := "/tmp/test-raioz-home"
	os.Setenv("RAIOZ_HOME", testDir)
	defer os.RemoveAll(testDir)

	base, err := GetBaseDir()
	if err != nil {
		t.Fatalf("GetBaseDir() error = %v", err)
	}
	if base != testDir {
		t.Errorf("GetBaseDir() = %v, want %v", base, testDir)
	}

	// Test 2: Without RAIOZ_HOME (will try /opt or fallback)
	os.Unsetenv("RAIOZ_HOME")

	base, err = GetBaseDir()
	if err != nil {
		t.Fatalf("GetBaseDir() error = %v", err)
	}

	// Should be either /opt/raioz-proyecto or fallback (~/.raioz)
	expectedOpt := "/opt/raioz-proyecto"
	usr, _ := user.Current()
	expectedFallback := filepath.Join(usr.HomeDir, ".raioz")

	if base != expectedOpt && base != expectedFallback {
		t.Errorf("GetBaseDir() = %v, want %v or %v", base, expectedOpt, expectedFallback)
	}
}

func TestGetFallbackBaseDir(t *testing.T) {
	fallback, err := getFallbackBaseDir()
	if err != nil {
		t.Fatalf("getFallbackBaseDir() error = %v", err)
	}

	usr, err := user.Current()
	if err != nil {
		t.Fatalf("user.Current() error = %v", err)
	}

	expected := filepath.Join(usr.HomeDir, ".raioz")
	if runtime.GOOS == "windows" {
		expected = filepath.Join(usr.HomeDir, ".raioz")
	}

	if fallback != expected {
		t.Errorf("getFallbackBaseDir() = %v, want %v", fallback, expected)
	}
}

func TestResolve(t *testing.T) {
	// Save original RAIOZ_HOME
	originalHome := os.Getenv("RAIOZ_HOME")
	defer os.Setenv("RAIOZ_HOME", originalHome)

	// Use test directory
	testDir := "/tmp/test-raioz-workspace"
	os.Setenv("RAIOZ_HOME", testDir)
	defer os.RemoveAll(testDir)

	ws, err := Resolve("test-project")
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}

	if ws == nil {
		t.Fatal("Workspace is nil")
	}

	if ws.Root == "" {
		t.Error("Workspace Root should not be empty")
	}

	if ws.ServicesDir == "" {
		t.Error("Workspace ServicesDir should not be empty")
	}

	if ws.EnvDir == "" {
		t.Error("Workspace EnvDir should not be empty")
	}

	// Verify structure
	expectedRoot := filepath.Join(testDir, "workspaces", "test-project")
	if ws.Root != expectedRoot {
		t.Errorf("Workspace Root = %v, want %v", ws.Root, expectedRoot)
	}

	expectedServices := filepath.Join(testDir, "services")
	if ws.ServicesDir != expectedServices {
		t.Errorf("Workspace ServicesDir = %v, want %v", ws.ServicesDir, expectedServices)
	}

	expectedEnv := filepath.Join(testDir, "env")
	if ws.EnvDir != expectedEnv {
		t.Errorf("Workspace EnvDir = %v, want %v", ws.EnvDir, expectedEnv)
	}

	// Verify directories exist
	if _, err := os.Stat(ws.Root); os.IsNotExist(err) {
		t.Errorf("Workspace root directory does not exist: %v", ws.Root)
	}

	if _, err := os.Stat(ws.ServicesDir); os.IsNotExist(err) {
		t.Errorf("Services directory does not exist: %v", ws.ServicesDir)
	}

	if _, err := os.Stat(ws.EnvDir); os.IsNotExist(err) {
		t.Errorf("Env directory does not exist: %v", ws.EnvDir)
	}

	// Verify env subdirectories exist
	envServices := filepath.Join(ws.EnvDir, "services")
	envProjects := filepath.Join(ws.EnvDir, "projects")

	if _, err := os.Stat(envServices); os.IsNotExist(err) {
		t.Errorf("Env services directory does not exist: %v", envServices)
	}

	if _, err := os.Stat(envProjects); os.IsNotExist(err) {
		t.Errorf("Env projects directory does not exist: %v", envProjects)
	}
}

func TestGetBaseDirFromWorkspace(t *testing.T) {
	ws := &Workspace{
		Root:        "/tmp/base/workspaces/test-project",
		ServicesDir: "/tmp/base/services",
		EnvDir:      "/tmp/base/env",
	}

	baseDir := GetBaseDirFromWorkspace(ws)
	expected := "/tmp/base"

	if baseDir != expected {
		t.Errorf("GetBaseDirFromWorkspace() = %v, want %v", baseDir, expected)
	}
}

func TestGetStatePath(t *testing.T) {
	ws := &Workspace{
		Root:        "/tmp/test",
		ServicesDir: "/tmp/services",
	}

	expected := filepath.Join("/tmp/test", stateFileName)
	actual := GetStatePath(ws)

	if actual != expected {
		t.Errorf("GetStatePath() = %v, want %v", actual, expected)
	}
}

func TestGetComposePath(t *testing.T) {
	ws := &Workspace{
		Root:        "/tmp/test",
		ServicesDir: "/tmp/services",
	}

	expected := filepath.Join("/tmp/test", composeFileName)
	actual := GetComposePath(ws)

	if actual != expected {
		t.Errorf("GetComposePath() = %v, want %v", actual, expected)
	}
}

func TestGetEnvDir(t *testing.T) {
	ws := &Workspace{
		Root:        "/tmp/test",
		ServicesDir: "/tmp/services",
		EnvDir:      "/tmp/env",
	}

	expected := "/tmp/env"
	actual := GetEnvDir(ws)

	if actual != expected {
		t.Errorf("GetEnvDir() = %v, want %v", actual, expected)
	}
}

func TestGetLocalServicesDir(t *testing.T) {
	ws := &Workspace{
		Root:             "/tmp/test",
		ServicesDir:      "/tmp/services",
		LocalServicesDir: "/tmp/test/local",
	}

	expected := "/tmp/test/local"
	actual := GetLocalServicesDir(ws)

	if actual != expected {
		t.Errorf("GetLocalServicesDir() = %v, want %v", actual, expected)
	}
}

func TestGetReadonlyServicesDir(t *testing.T) {
	ws := &Workspace{
		Root:                "/tmp/test",
		ServicesDir:        "/tmp/services",
		ReadonlyServicesDir: "/tmp/test/readonly",
	}

	expected := "/tmp/test/readonly"
	actual := GetReadonlyServicesDir(ws)

	if actual != expected {
		t.Errorf("GetReadonlyServicesDir() = %v, want %v", actual, expected)
	}
}
