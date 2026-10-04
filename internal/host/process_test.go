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

// Reproduces the process-group teardown gap: a shell launcher backgrounds a real worker
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

	// Signal delivery + kernel reap aren't instantaneous; poll until
	// the grandchild is gone or the (generous) deadline expires.
	deadline := time.Now().Add(2 * time.Second)
	for IsProcessAlive(workerPID) && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if IsProcessAlive(workerPID) {
		t.Errorf("grandchild sleep pid=%d survived stop",
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
