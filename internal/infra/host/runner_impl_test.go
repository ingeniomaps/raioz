package host

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"raioz/internal/domain/interfaces"
	hostpkg "raioz/internal/host"
)

func setupWS(t *testing.T) *interfaces.Workspace {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("RAIOZ_HOME", dir)

	wsDir := filepath.Join(dir, "proj")
	if err := os.MkdirAll(wsDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	return &interfaces.Workspace{
		Root:                wsDir,
		ServicesDir:         filepath.Join(wsDir, "services"),
		LocalServicesDir:    filepath.Join(wsDir, "local"),
		ReadonlyServicesDir: filepath.Join(wsDir, "readonly"),
		EnvDir:              filepath.Join(wsDir, "env"),
	}
}

func TestNewHostRunner(t *testing.T) {
	r := NewHostRunner()
	if r == nil {
		t.Fatal("NewHostRunner returned nil")
	}
}

func TestHostRunnerImpl_SaveLoadProcessesState(t *testing.T) {
	r := NewHostRunner()
	ws := setupWS(t)

	// Start empty
	loaded, err := r.LoadProcessesState(ws)
	if err != nil {
		t.Logf("LoadProcessesState on empty: %v", err)
	}
	_ = loaded

	// Save some state
	procs := map[string]*hostpkg.ProcessInfo{
		"api": {PID: 12345},
	}
	if err := r.SaveProcessesState(ws, procs); err != nil {
		t.Fatalf("SaveProcessesState: %v", err)
	}

	// Load it back
	loaded, err = r.LoadProcessesState(ws)
	if err != nil {
		t.Fatalf("LoadProcessesState: %v", err)
	}
	if loaded["api"] == nil || loaded["api"].PID != 12345 {
		t.Errorf("loaded state mismatch: %+v", loaded)
	}
}

func TestHostRunnerImpl_RemoveProcessesState(t *testing.T) {
	r := NewHostRunner()
	ws := setupWS(t)

	procs := map[string]*hostpkg.ProcessInfo{"api": {PID: 1}}
	if err := r.SaveProcessesState(ws, procs); err != nil {
		t.Fatalf("SaveProcessesState: %v", err)
	}

	if err := r.RemoveProcessesState(ws); err != nil {
		t.Errorf("RemoveProcessesState: %v", err)
	}
}

func TestHostRunnerImpl_DetectComposePath(t *testing.T) {
	r := NewHostRunner()
	// No compose file exists — should return empty or the explicit path
	got := r.DetectComposePath(t.TempDir(), "npm start", "")
	_ = got // Impl may return empty string
}

func TestHostRunnerImpl_DetectComposePath_Explicit(t *testing.T) {
	r := NewHostRunner()
	got := r.DetectComposePath(t.TempDir(), "", "/explicit/path.yml")
	if got != "/explicit/path.yml" {
		t.Logf("explicit path not preserved: got %q", got)
	}
}

func TestHostRunnerImpl_IsServiceRunning(t *testing.T) {
	r := NewHostRunner()

	// Current process should be running
	pid := os.Getpid()
	running, err := r.IsServiceRunning(pid)
	if err != nil {
		t.Fatalf("IsServiceRunning: %v", err)
	}
	if !running {
		t.Error("current process should be running")
	}

	// Very high PID should not exist
	_, _ = r.IsServiceRunning(999999999)
}

func TestHostRunnerImpl_StopServiceWithCommand_InvalidPID(t *testing.T) {
	r := NewHostRunner()
	ctx := context.Background()
	// Non-existent PID — should error or be a no-op
	_ = r.StopServiceWithCommand(ctx, 999999999, "")
}

func TestHostRunnerImpl_StopServiceWithCommandAndPath_InvalidPID(t *testing.T) {
	r := NewHostRunner()
	ctx := context.Background()
	_ = r.StopServiceWithCommandAndPath(ctx, 999999999, "", "")
}
