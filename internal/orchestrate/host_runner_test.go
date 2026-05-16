package orchestrate

import (
	"context"
	"os"
	"os/exec"
	"testing"
	"time"

	"raioz/internal/domain/interfaces"
	"raioz/internal/domain/models"
	"raioz/internal/naming"
)

// Issue 035 — once Start returns, the spawned process must NOT be
// killed when the parent context cancels. Previous behavior used
// exec.CommandContext, which SIGKILLed the launcher mid-build when
// raioz exited normally and the cobra signal context's deferred
// stop() ran.
func TestHostRunner_Start_SubprocessSurvivesParentCtxCancel(t *testing.T) {
	if _, err := exec.LookPath("sleep"); err != nil {
		t.Skip("sleep not available")
	}

	r := &HostRunner{}
	dir := t.TempDir()
	svc := interfaces.ServiceContext{
		Name:        "survivor",
		Path:        dir,
		ProjectName: "host-survive-" + t.Name(),
		Detection: models.DetectResult{
			Runtime:      models.RuntimeMake,
			StartCommand: "sleep 5",
		},
	}
	t.Cleanup(func() { os.RemoveAll(naming.TempDir(svc.ProjectName)) })
	t.Cleanup(func() {
		// Best-effort cleanup in case the test fails before Stop.
		_ = r.Stop(context.Background(), svc)
	})

	// Use a cancelable ctx and cancel it immediately AFTER Start
	// returns. Asserts that the child outlives ctx cancel.
	ctx, cancel := context.WithCancel(context.Background())
	if err := r.Start(ctx, svc); err != nil {
		t.Fatalf("Start: %v", err)
	}
	pid := r.GetPID("survivor")
	if pid == 0 {
		t.Fatal("expected non-zero PID")
	}

	cancel()
	// Give exec.Cmd's would-be killer a moment to fire if the ctx
	// link were still in place. 500ms is generous — the previous
	// (broken) behavior reaped within tens of milliseconds.
	time.Sleep(500 * time.Millisecond)

	// Process must still exist.
	if proc, err := os.FindProcess(pid); err != nil || proc == nil {
		t.Fatalf("child gone after parent ctx cancel (pid=%d): %v", pid, err)
	}
	if err := exec.Command("kill", "-0", fmtPID(pid)).Run(); err != nil {
		t.Errorf("kill -0 pid=%d failed after ctx cancel: %v", pid, err)
	}
}

// fmtPID is here so the test stays Bash-portable on macOS/Linux
// without pulling strconv.
func fmtPID(pid int) string {
	const digits = "0123456789"
	if pid == 0 {
		return "0"
	}
	var out []byte
	for pid > 0 {
		out = append([]byte{digits[pid%10]}, out...)
		pid /= 10
	}
	return string(out)
}

func TestHostRunner_Start_NoCommand(t *testing.T) {
	r := &HostRunner{}
	svc := interfaces.ServiceContext{
		Name:        "api",
		ProjectName: "host-nocmd-" + t.Name(),
		Detection:   models.DetectResult{Runtime: models.RuntimeGo},
	}
	t.Cleanup(func() { os.RemoveAll(naming.TempDir(svc.ProjectName)) })

	err := r.Start(context.Background(), svc)
	if err == nil {
		t.Error("expected error for missing command")
	}
}

func TestHostRunner_Status_Unknown(t *testing.T) {
	r := &HostRunner{}
	svc := interfaces.ServiceContext{Name: "api"}

	status, err := r.Status(context.Background(), svc)
	if err != nil {
		t.Errorf("Status: %v", err)
	}
	if status != "stopped" {
		t.Errorf("expected stopped, got %s", status)
	}
}

func TestHostRunner_Status_NotTracked(t *testing.T) {
	r := &HostRunner{}
	r.SetPID("other", 12345)
	svc := interfaces.ServiceContext{Name: "api"}

	status, err := r.Status(context.Background(), svc)
	if err != nil {
		t.Errorf("Status: %v", err)
	}
	if status != "stopped" {
		t.Errorf("expected stopped, got %s", status)
	}
}

func TestHostRunner_Status_DeadProcess(t *testing.T) {
	r := &HostRunner{}
	// Pick a likely-unused PID
	r.SetPID("api", 999999)
	svc := interfaces.ServiceContext{Name: "api"}

	status, _ := r.Status(context.Background(), svc)
	if status != "stopped" {
		t.Errorf("expected stopped for dead pid, got %s", status)
	}
}

func TestHostRunner_Stop_NoProcesses(t *testing.T) {
	r := &HostRunner{}
	svc := interfaces.ServiceContext{Name: "api"}

	if err := r.Stop(context.Background(), svc); err != nil {
		t.Errorf("Stop: %v", err)
	}
}

func TestHostRunner_Stop_NotTracked(t *testing.T) {
	r := &HostRunner{}
	r.SetPID("other", 1)
	svc := interfaces.ServiceContext{Name: "api"}

	if err := r.Stop(context.Background(), svc); err != nil {
		t.Errorf("Stop: %v", err)
	}
}

func TestHostRunner_Stop_DeadProcess(t *testing.T) {
	r := &HostRunner{}
	// Use a PID unlikely to exist
	r.SetPID("api", 999999)
	svc := interfaces.ServiceContext{Name: "api"}

	// Should handle gracefully and remove the entry
	_ = r.Stop(context.Background(), svc)
	if pid := r.GetPID("api"); pid != 0 {
		t.Errorf("expected pid cleared, got %d", pid)
	}
}

func TestHostRunner_StartStopSleep(t *testing.T) {
	// Verify we can start a real process (sleep) and stop it cleanly.
	if _, err := exec.LookPath("sleep"); err != nil {
		t.Skip("sleep not available")
	}

	r := &HostRunner{}
	dir := t.TempDir()
	svc := interfaces.ServiceContext{
		Name:        "sleeper",
		Path:        dir,
		ProjectName: "host-sleep-" + t.Name(),
		Detection: models.DetectResult{
			Runtime:      models.RuntimeMake,
			StartCommand: "sleep 30",
		},
	}
	t.Cleanup(func() { os.RemoveAll(naming.TempDir(svc.ProjectName)) })

	ctx := context.Background()
	if err := r.Start(ctx, svc); err != nil {
		t.Fatalf("Start: %v", err)
	}

	if pid := r.GetPID("sleeper"); pid == 0 {
		t.Error("expected non-zero PID")
	}

	// Give the process a moment to settle
	time.Sleep(100 * time.Millisecond)

	status, _ := r.Status(ctx, svc)
	if status != "running" {
		t.Errorf("expected running, got %s", status)
	}

	if err := r.Stop(ctx, svc); err != nil {
		t.Errorf("Stop: %v", err)
	}

	if pid := r.GetPID("sleeper"); pid != 0 {
		t.Errorf("expected PID cleared after stop, got %d", pid)
	}
}

func TestHostRunner_Start_PrefersDevCommand(t *testing.T) {
	if _, err := exec.LookPath("sleep"); err != nil {
		t.Skip("sleep not available")
	}

	r := &HostRunner{}
	dir := t.TempDir()
	svc := interfaces.ServiceContext{
		Name:        "sleeper2",
		Path:        dir,
		ProjectName: "host-dev-" + t.Name(),
		Detection: models.DetectResult{
			Runtime:      models.RuntimeNPM,
			DevCommand:   "sleep 30",
			StartCommand: "nope-bad-cmd-xyz",
		},
	}
	t.Cleanup(func() { os.RemoveAll(naming.TempDir(svc.ProjectName)) })
	t.Cleanup(func() { _ = r.Stop(context.Background(), svc) })

	if err := r.Start(context.Background(), svc); err != nil {
		t.Errorf("Start: %v", err)
	}
	if r.GetPID("sleeper2") == 0 {
		t.Error("expected non-zero PID")
	}
}
