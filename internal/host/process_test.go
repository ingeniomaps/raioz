package host

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"

	"raioz/internal/domain/models"
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

// Reproduces issue 019: a shell launcher backgrounds a real worker
// (`sleep 60 &; wait`) so the worker is a grandchild that shares the
// shell's process group but is NOT killed when the shell receives
// SIGTERM directly. Stop must reach the whole group so the worker
// dies and frees its resources (port, etc.).
func TestStopServiceWithCommandKillsProcessGroup(t *testing.T) {
	skipIfNoBinary(t, "sh")
	skipIfNoBinary(t, "sleep")

	cmd := exec.Command("sh", "-c", "sleep 60 & wait")
	SetNewProcessGroup(cmd)
	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	leaderPID := cmd.Process.Pid
	t.Cleanup(func() {
		_ = ForceKillProcessTree(leaderPID)
	})
	time.Sleep(100 * time.Millisecond)

	// Find the grandchild's PID so the assertion can target it
	// independently of the shell leader.
	workerPID := findChildSleepPID(t, leaderPID)
	if workerPID == 0 {
		t.Skip("could not locate grandchild sleep; /proc not available")
	}

	if err := StopServiceWithCommand(context.Background(), leaderPID, ""); err != nil {
		t.Fatalf("StopServiceWithCommand: %v", err)
	}

	if IsProcessAlive(workerPID) {
		t.Errorf("grandchild sleep pid=%d survived stop — issue 019 regression",
			workerPID)
	}
}

// findChildSleepPID walks /proc looking for a process whose PPID is in
// the same group as leaderPID and whose comm contains "sleep". Returns
// 0 on non-Linux or when no match is found.
func findChildSleepPID(t *testing.T, leaderPID int) int {
	t.Helper()
	if runtime.GOOS != "linux" {
		return 0
	}
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return 0
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		comm, err := os.ReadFile("/proc/" + e.Name() + "/comm")
		if err != nil || !strings.Contains(string(comm), "sleep") {
			continue
		}
		stat, err := os.ReadFile("/proc/" + e.Name() + "/stat")
		if err != nil {
			continue
		}
		// /proc/<pid>/stat: pid (comm) state ppid ...
		// Find the ')' that closes comm, then split.
		raw := string(stat)
		end := strings.LastIndex(raw, ")")
		if end < 0 || end+1 >= len(raw) {
			continue
		}
		fields := strings.Fields(raw[end+1:])
		if len(fields) < 2 {
			continue
		}
		// fields[0] = state, fields[1] = ppid.
		var ppid int
		if _, err := fmt.Sscanf(fields[1], "%d", &ppid); err != nil {
			continue
		}
		if ppid == leaderPID {
			var pid int
			if _, err := fmt.Sscanf(e.Name(), "%d", &pid); err == nil {
				return pid
			}
		}
	}
	return 0
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
	deps := &models.Deps{Project: models.Project{Name: "test"}}

	svc := models.Service{
		Source: models.SourceConfig{
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
	deps := &models.Deps{Project: models.Project{Name: "test"}}

	svc := models.Service{
		Source: models.SourceConfig{
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
	deps := &models.Deps{Project: models.Project{Name: "test"}}

	svc := models.Service{
		Source: models.SourceConfig{
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
	deps := &models.Deps{Project: models.Project{Name: "test"}}

	svc := models.Service{
		Source: models.SourceConfig{Kind: "local", Path: "."},
	}

	_, err := StartService(context.Background(), ws, deps, "svc", svc, t.TempDir())
	if err == nil {
		t.Errorf("expected error for missing command")
	}
}

func TestStartServiceImageRejected(t *testing.T) {
	ws := &workspace.Workspace{Root: t.TempDir()}
	deps := &models.Deps{Project: models.Project{Name: "test"}}

	svc := models.Service{
		Source: models.SourceConfig{Kind: "image", Image: "nginx", Command: "nginx -g daemon off;"},
	}

	_, err := StartService(context.Background(), ws, deps, "svc", svc, "")
	if err == nil {
		t.Errorf("expected error for image kind")
	}
}

func TestStartServiceLocalMissingPath(t *testing.T) {
	ws := &workspace.Workspace{Root: t.TempDir()}
	deps := &models.Deps{Project: models.Project{Name: "test"}}

	svc := models.Service{
		Source: models.SourceConfig{
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
	deps := &models.Deps{Project: models.Project{Name: "test"}}

	svc := models.Service{
		Source: models.SourceConfig{
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

// A background command that fork+exec'd ok but exits immediately
// (e.g. port already bound) must surface as an error from StartService.
// Without the settle window the start was reported as "success" and the
// caller had no way to tell the service was already dead.
func TestStartServiceEarlyExitDetected(t *testing.T) {
	skipIfNoBinary(t, "false")

	// Shrink the settle window so the test stays snappy. `false` exits
	// instantly so 200 ms is plenty for cmd.Wait() to fire first.
	prev := startSettleWindow
	startSettleWindow = 200 * time.Millisecond
	t.Cleanup(func() { startSettleWindow = prev })

	ws := &workspace.Workspace{Root: t.TempDir()}
	deps := &models.Deps{Project: models.Project{Name: "test"}}
	svc := models.Service{
		Source: models.SourceConfig{
			Kind:    "local",
			Path:    ".",
			Command: "false",
		},
	}

	_, err := StartService(context.Background(), ws, deps, "svc", svc, t.TempDir())
	if err == nil {
		t.Fatal("expected error when process exits immediately, got nil")
	}
	if !strings.Contains(err.Error(), "exited within") {
		t.Errorf("error should mention early exit, got: %v", err)
	}
}

// When the service writes to stderr before crashing,
// the returned error must include the tail so the user can diagnose
// without having to grep through log files.
func TestStartServiceEarlyExitIncludesStderrTail(t *testing.T) {
	skipIfNoBinary(t, "sh")

	prev := startSettleWindow
	startSettleWindow = 300 * time.Millisecond
	t.Cleanup(func() { startSettleWindow = prev })

	// Stage a script on disk so we don't fight parseCommand's space-split
	// (which doesn't honor quotes). No `.sh` suffix on purpose: that would
	// route through shouldWaitForCommand's synchronous path and skip the
	// settle window entirely.
	dir := t.TempDir()
	script := dir + "/crash"
	body := "#!/bin/sh\necho bind:port-already-in-use 1>&2\nexit 1\n"
	if err := os.WriteFile(script, []byte(body), 0755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	ws := &workspace.Workspace{Root: t.TempDir()}
	deps := &models.Deps{Project: models.Project{Name: "test"}}
	svc := models.Service{
		Source: models.SourceConfig{
			Kind:    "local",
			Path:    ".",
			Command: script,
		},
	}

	_, err := StartService(context.Background(), ws, deps, "svc", svc, dir)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "stderr tail") {
		t.Errorf("error should include stderr-tail block, got: %v", err)
	}
	if !strings.Contains(err.Error(), "bind:port-already-in-use") {
		t.Errorf("error should embed the stderr line, got: %v", err)
	}
}

// A long-running service must NOT be misclassified as
// early-exit. The settle window is short on purpose; this is the
// regression test for the obvious false-positive direction.
func TestStartServiceSurvivesSettleWindow(t *testing.T) {
	skipIfNoBinary(t, "sleep")

	prev := startSettleWindow
	startSettleWindow = 100 * time.Millisecond
	t.Cleanup(func() { startSettleWindow = prev })

	ws := &workspace.Workspace{Root: t.TempDir()}
	deps := &models.Deps{Project: models.Project{Name: "test"}}
	svc := models.Service{
		Source: models.SourceConfig{
			Kind:    "local",
			Path:    ".",
			Command: "sleep 30",
		},
	}

	info, err := StartService(context.Background(), ws, deps, "svc", svc, t.TempDir())
	if err != nil {
		t.Fatalf("StartService should succeed for surviving process, got %v", err)
	}
	t.Cleanup(func() {
		proc, _ := os.FindProcess(info.PID)
		if proc != nil {
			_ = proc.Kill()
		}
	})

	if info == nil || info.PID == 0 {
		t.Fatalf("expected valid PID after settle window, got %+v", info)
	}
}

// Launcher / early-exit coexistence: a launcher that exits cleanly (exit 0)
// inside the settle window must NOT be flagged as early-exit. The classic
// shape is `./launch.sh` that does `docker run -d` and returns 0.
func TestStartServiceCleanExitInSettleWindowIsNotError(t *testing.T) {
	skipIfNoBinary(t, "true")

	prev := startSettleWindow
	startSettleWindow = 200 * time.Millisecond
	t.Cleanup(func() { startSettleWindow = prev })

	dir := t.TempDir()
	script := dir + "/launcher"
	body := "#!/bin/sh\nexit 0\n"
	if err := os.WriteFile(script, []byte(body), 0755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	ws := &workspace.Workspace{Root: t.TempDir()}
	deps := &models.Deps{Project: models.Project{Name: "test"}}
	svc := models.Service{
		Source: models.SourceConfig{
			Kind:    "local",
			Path:    ".",
			Command: script,
		},
	}

	info, err := StartService(context.Background(), ws, deps, "svc", svc, dir)
	if err != nil {
		t.Fatalf("clean exit must not error: %v", err)
	}
	if info == nil {
		t.Fatal("expected ProcessInfo on clean detach, got nil")
	}
}

func TestStartServiceSynchronousTrue(t *testing.T) {
	skipIfNoBinary(t, "true")

	ws := &workspace.Workspace{Root: t.TempDir()}
	deps := &models.Deps{Project: models.Project{Name: "test"}}

	svc := models.Service{
		Source: models.SourceConfig{
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
