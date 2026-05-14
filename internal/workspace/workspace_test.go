package workspace

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestGetBaseDir(t *testing.T) {
	t.Run("RAIOZ_HOME wins", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "raioz-home")
		t.Setenv("RAIOZ_HOME", dir)
		t.Setenv("XDG_STATE_HOME", "")

		base, err := GetBaseDir()
		if err != nil {
			t.Fatalf("GetBaseDir() error = %v", err)
		}
		if base != dir {
			t.Errorf("GetBaseDir() = %q, want %q", base, dir)
		}
	})

	t.Run("XDG_STATE_HOME used when RAIOZ_HOME unset", func(t *testing.T) {
		xdg := t.TempDir()
		t.Setenv("RAIOZ_HOME", "")
		t.Setenv("XDG_STATE_HOME", xdg)

		base, err := GetBaseDir()
		if err != nil {
			t.Fatalf("GetBaseDir() error = %v", err)
		}
		want := filepath.Join(xdg, "raioz")
		if base != want {
			t.Errorf("GetBaseDir() = %q, want %q", base, want)
		}
	})

	t.Run("home fallback when nothing set", func(t *testing.T) {
		t.Setenv("RAIOZ_HOME", "")
		t.Setenv("XDG_STATE_HOME", "")

		base, err := GetBaseDir()
		if err != nil {
			t.Fatalf("GetBaseDir() error = %v", err)
		}
		home, herr := os.UserHomeDir()
		if herr != nil || home == "" {
			t.Skip("UserHomeDir() unavailable; skip platform-specific assertion")
		}
		want := filepath.Join(home, ".local", "state", "raioz")
		if base != want {
			t.Errorf("GetBaseDir() = %q, want %q", base, want)
		}
	})
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

	expectedEnv := filepath.Join(testDir, "workspaces", "test-project", "env")
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

func TestResolveEdgeCases(t *testing.T) {
	originalHome := os.Getenv("RAIOZ_HOME")
	defer os.Setenv("RAIOZ_HOME", originalHome)

	t.Run("empty project name", func(t *testing.T) {
		tmpDir := t.TempDir()
		os.Setenv("RAIOZ_HOME", tmpDir)

		ws, err := Resolve("")
		if err != nil {
			t.Logf("Resolve with empty name returned error: %v", err)
		}
		if ws != nil {
			// Workspace was created, which is valid behavior
		}
	})

	t.Run("project name with special characters", func(t *testing.T) {
		tmpDir := t.TempDir()
		os.Setenv("RAIOZ_HOME", tmpDir)

		ws, err := Resolve("test-project_123-456")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if ws == nil {
			t.Fatal("Workspace should not be nil")
		}
	})

	t.Run("very long project name", func(t *testing.T) {
		tmpDir := t.TempDir()
		os.Setenv("RAIOZ_HOME", tmpDir)

		longName := "a" + strings.Repeat("b", 200)
		ws, err := Resolve(longName)
		if err != nil {
			t.Logf("Resolve with very long name returned error (acceptable): %v", err)
			return
		}
		if ws == nil {
			t.Fatal("Workspace should not be nil")
		}
	})
}

func TestGetBaseDirFromWorkspace(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("docs/issues/068: assertion uses Unix path separators")
	}
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
		ServicesDir:         "/tmp/services",
		ReadonlyServicesDir: "/tmp/test/readonly",
	}

	expected := "/tmp/test/readonly"
	actual := GetReadonlyServicesDir(ws)

	if actual != expected {
		t.Errorf("GetReadonlyServicesDir() = %v, want %v", actual, expected)
	}
}
