//go:build linux

package orchestrate

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"raioz/internal/domain/interfaces"
	"raioz/internal/domain/models"
	"raioz/internal/naming"
)

// logFDIsOpen reports whether /proc/self/fd contains a symlink pointing at
// the given log path. Used by the leak regression so we can spot the
// specific fd we're tracking instead of relying on a bulk count.
func logFDIsOpen(t *testing.T, logPath string) bool {
	t.Helper()
	entries, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		t.Fatalf("read /proc/self/fd: %v", err)
	}
	for _, e := range entries {
		fd, _ := strconv.Atoi(e.Name())
		if fd == 0 {
			continue
		}
		target, err := os.Readlink(filepath.Join("/proc/self/fd", e.Name()))
		if err != nil {
			continue
		}
		if target == logPath {
			return true
		}
	}
	return false
}

// TestHostRunner_Start_ClosesLogFileAfterExit pins ADR-034: after a host
// service's process exits, the parent's copy of the log fd must be
// released. Previously the file was held until GC ran the finalizer,
// accumulating one leaked fd per Start in long watch-mode sessions.
func TestHostRunner_Start_ClosesLogFileAfterExit(t *testing.T) {
	if _, err := exec.LookPath("sleep"); err != nil {
		t.Skip("sleep not available")
	}

	r := &HostRunner{}
	dir := t.TempDir()
	svc := interfaces.ServiceContext{
		Name:        "quick",
		Path:        dir,
		ProjectName: "host-fd-" + t.Name(),
		Detection: models.DetectResult{
			Runtime: models.RuntimeMake,
			// Stays alive long enough to survive the settle window,
			// then exits quickly so the test doesn't take forever.
			// GNU coreutils sleep accepts fractional seconds.
			StartCommand: "sleep 1",
		},
	}
	logPath := naming.LogFile(svc.ProjectName, svc.Name)
	t.Cleanup(func() { os.RemoveAll(naming.TempDir(svc.ProjectName)) })

	if err := r.Start(context.Background(), svc); err != nil {
		t.Fatalf("Start: %v", err)
	}

	if !logFDIsOpen(t, logPath) {
		t.Fatal("expected log fd to be open while child is running")
	}

	// Wait until the child exits and the drain goroutine has had a chance
	// to release the fd. Poll instead of a fixed sleep so the test isn't
	// flaky on a busy CI runner.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if !logFDIsOpen(t, logPath) {
			return // success
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Errorf("log fd still open after child exited; drain goroutine did not close it")
}
