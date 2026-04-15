package orchestrate

import (
	"context"
	"os"
	"os/exec"
	"testing"
	"time"

	"raioz/internal/detect"
	"raioz/internal/domain/interfaces"
	"raioz/internal/naming"
)

func TestHostRunner_Start_NoCommand(t *testing.T) {
	r := &HostRunner{}
	svc := interfaces.ServiceContext{
		Name:        "api",
		ProjectName: "host-nocmd-" + t.Name(),
		Detection:   detect.DetectResult{Runtime: detect.RuntimeGo},
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
		Detection: detect.DetectResult{
			Runtime:      detect.RuntimeMake,
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
		Detection: detect.DetectResult{
			Runtime:      detect.RuntimeNPM,
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
