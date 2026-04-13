package host

import (
	"context"
	"os"
	"os/exec"
	"runtime"
	"testing"
	"time"

	"raioz/internal/config"
	"raioz/internal/workspace"
)

func skipIfNoBinary(t *testing.T, name string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows")
	}
	if _, err := exec.LookPath(name); err != nil {
		t.Skipf("%s not available: %v", name, err)
	}
}

func TestIsServiceRunningAlive(t *testing.T) {
	skipIfNoBinary(t, "sleep")

	cmd := exec.Command("sleep", "30")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start sleep: %v", err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	})

	running, err := IsServiceRunning(cmd.Process.Pid)
	if err != nil {
		t.Errorf("IsServiceRunning() error = %v", err)
	}
	if !running {
		t.Errorf("IsServiceRunning() = false, want true")
	}
}

func TestIsServiceRunningDead(t *testing.T) {
	skipIfNoBinary(t, "true")

	cmd := exec.Command("true")
	if err := cmd.Run(); err != nil {
		t.Fatalf("run true: %v", err)
	}
	// Wait for process to be fully reaped
	pid := cmd.Process.Pid

	running, _ := IsServiceRunning(pid)
	if running {
		t.Errorf("IsServiceRunning(dead) = true, want false")
	}
}

func TestStopServiceWithCommandKillsProcess(t *testing.T) {
	skipIfNoBinary(t, "sleep")

	cmd := exec.Command("sleep", "60")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	pid := cmd.Process.Pid
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
	})

	// Give the process a moment to start
	time.Sleep(50 * time.Millisecond)

	ctx := context.Background()
	if err := StopServiceWithCommand(ctx, pid, ""); err != nil {
		t.Errorf("StopServiceWithCommand() error = %v", err)
	}

	// Process should be gone
	time.Sleep(100 * time.Millisecond)
	running, _ := IsServiceRunning(pid)
	if running {
		t.Errorf("process still running after stop")
	}
}

func TestStopServiceWithCommandAndPathInvalidPID(t *testing.T) {
	// Use a PID that is highly unlikely to exist
	ctx := context.Background()
	// Stopping a non-existent process via signal — on Linux FindProcess always
	// succeeds, but sending SIGTERM should fail or return "already finished".
	// Either way the function should not panic.
	_ = StopServiceWithCommandAndPath(ctx, 0, "", "")
}

func TestStopServiceWithCustomStopCommand(t *testing.T) {
	skipIfNoBinary(t, "true")

	dir := t.TempDir()
	ctx := context.Background()

	// Custom stop command "true" succeeds, pid 0 means no PID to track
	err := StopServiceWithCommandAndPath(ctx, 0, "true", dir)
	if err != nil {
		t.Errorf("StopServiceWithCommandAndPath() error = %v", err)
	}
}

func TestStopServiceWithFailingStopCommand(t *testing.T) {
	skipIfNoBinary(t, "sleep")

	// Start a real process
	cmd := exec.Command("sleep", "60")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	pid := cmd.Process.Pid
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
	})
	time.Sleep(50 * time.Millisecond)

	ctx := context.Background()
	// Stop command "false" fails, should fall back to SIGTERM on pid
	err := StopServiceWithCommandAndPath(ctx, pid, "false", "")
	if err != nil {
		t.Errorf("fallback stop error = %v", err)
	}

	time.Sleep(100 * time.Millisecond)
	if running, _ := IsServiceRunning(pid); running {
		t.Errorf("process still running after fallback stop")
	}
}

func TestStopServiceWrapper(t *testing.T) {
	// Deprecated wrapper, just exercise it with a sleeping process
	skipIfNoBinary(t, "sleep")

	cmd := exec.Command("sleep", "30")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	pid := cmd.Process.Pid
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
	})
	time.Sleep(50 * time.Millisecond)

	if err := StopService(context.Background(), pid); err != nil {
		t.Errorf("StopService() error = %v", err)
	}
}

func TestStartServiceLocalBackground(t *testing.T) {
	skipIfNoBinary(t, "sleep")

	projectDir := t.TempDir()
	ws := &workspace.Workspace{Root: t.TempDir()}
	deps := &config.Deps{Project: config.Project{Name: "test"}}

	svc := config.Service{
		Source: config.SourceConfig{
			Kind:    "local",
			Path:    ".",
			Command: "sleep 30",
		},
	}

	ctx := context.Background()
	info, err := StartService(ctx, ws, deps, "svc", svc, projectDir)
	if err != nil {
		t.Fatalf("StartService() error = %v", err)
	}
	if info == nil || info.PID == 0 {
		t.Fatalf("expected valid PID, got %+v", info)
	}
	t.Cleanup(func() {
		proc, _ := os.FindProcess(info.PID)
		if proc != nil {
			_ = proc.Kill()
		}
	})

	if info.Service != "svc" {
		t.Errorf("info.Service = %q, want %q", info.Service, "svc")
	}
	if info.Command != "sleep 30" {
		t.Errorf("info.Command = %q, want %q", info.Command, "sleep 30")
	}

	// Verify running
	time.Sleep(50 * time.Millisecond)
	running, _ := IsServiceRunning(info.PID)
	if !running {
		t.Errorf("process not running after StartService")
	}

	// Stop it
	if err := StopServiceWithCommand(ctx, info.PID, ""); err != nil {
		t.Errorf("stop: %v", err)
	}
}

func TestStartServiceLocalRelativePath(t *testing.T) {
	skipIfNoBinary(t, "sleep")

	projectDir := t.TempDir()
	sub := projectDir + "/sub"
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatalf("mkdir sub: %v", err)
	}
	ws := &workspace.Workspace{Root: t.TempDir()}
	deps := &config.Deps{Project: config.Project{Name: "test"}}

	svc := config.Service{
		Source: config.SourceConfig{
			Kind:    "local",
			Path:    "sub",
			Command: "sleep 30",
		},
	}

	ctx := context.Background()
	info, err := StartService(ctx, ws, deps, "svc", svc, projectDir)
	if err != nil {
		t.Fatalf("StartService() error = %v", err)
	}
	t.Cleanup(func() {
		proc, _ := os.FindProcess(info.PID)
		if proc != nil {
			_ = proc.Kill()
		}
	})

	if info.PID == 0 {
		t.Errorf("expected PID > 0")
	}
}

func TestStartServiceLocalAbsolutePath(t *testing.T) {
	skipIfNoBinary(t, "sleep")

	absPath := t.TempDir()
	ws := &workspace.Workspace{Root: t.TempDir()}
	deps := &config.Deps{Project: config.Project{Name: "test"}}

	svc := config.Service{
		Source: config.SourceConfig{
			Kind:    "local",
			Path:    absPath,
			Command: "sleep 30",
		},
	}

	ctx := context.Background()
	info, err := StartService(ctx, ws, deps, "svc", svc, "")
	if err != nil {
		t.Fatalf("StartService() error = %v", err)
	}
	t.Cleanup(func() {
		proc, _ := os.FindProcess(info.PID)
		if proc != nil {
			_ = proc.Kill()
		}
	})
}

func TestStartServiceMissingCommand(t *testing.T) {
	ws := &workspace.Workspace{Root: t.TempDir()}
	deps := &config.Deps{Project: config.Project{Name: "test"}}

	svc := config.Service{
		Source: config.SourceConfig{Kind: "local", Path: "."},
	}

	_, err := StartService(context.Background(), ws, deps, "svc", svc, t.TempDir())
	if err == nil {
		t.Errorf("expected error for missing command")
	}
}

func TestStartServiceImageRejected(t *testing.T) {
	ws := &workspace.Workspace{Root: t.TempDir()}
	deps := &config.Deps{Project: config.Project{Name: "test"}}

	svc := config.Service{
		Source: config.SourceConfig{Kind: "image", Image: "nginx", Command: "nginx -g daemon off;"},
	}

	_, err := StartService(context.Background(), ws, deps, "svc", svc, "")
	if err == nil {
		t.Errorf("expected error for image kind")
	}
}

func TestStartServiceLocalMissingPath(t *testing.T) {
	ws := &workspace.Workspace{Root: t.TempDir()}
	deps := &config.Deps{Project: config.Project{Name: "test"}}

	svc := config.Service{
		Source: config.SourceConfig{
			Kind:    "local",
			Path:    "/nonexistent/definitely/does/not/exist/xyz123",
			Command: "sleep 30",
		},
	}

	_, err := StartService(context.Background(), ws, deps, "svc", svc, "")
	if err == nil {
		t.Errorf("expected error for missing path")
	}
}

func TestStartServiceLocalInvalidCommand(t *testing.T) {
	ws := &workspace.Workspace{Root: t.TempDir()}
	deps := &config.Deps{Project: config.Project{Name: "test"}}

	svc := config.Service{
		Source: config.SourceConfig{
			Kind:    "local",
			Path:    ".",
			Command: "nonexistent-binary-xyz-12345",
		},
	}

	_, err := StartService(context.Background(), ws, deps, "svc", svc, t.TempDir())
	if err == nil {
		t.Errorf("expected error when binary does not exist")
	}
}

func TestStartServiceSynchronousTrue(t *testing.T) {
	skipIfNoBinary(t, "true")

	ws := &workspace.Workspace{Root: t.TempDir()}
	deps := &config.Deps{Project: config.Project{Name: "test"}}

	svc := config.Service{
		Source: config.SourceConfig{
			Kind:    "local",
			Path:    ".",
			Command: "./true.sh", // shouldWait returns true for ./ commands
		},
	}

	// Create a script named true.sh that exits 0
	projectDir := t.TempDir()
	scriptPath := projectDir + "/true.sh"
	if err := os.WriteFile(scriptPath, []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	info, err := StartService(context.Background(), ws, deps, "svc", svc, projectDir)
	if err != nil {
		t.Fatalf("StartService() error = %v", err)
	}
	if info.PID != 0 {
		t.Errorf("synchronous command should return PID=0, got %d", info.PID)
	}
}
