package orchestrate

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"raioz/internal/detect"
	"raioz/internal/domain/interfaces"
	"raioz/internal/naming"
)

// writeExecutable creates a small shell script at path and chmods it executable.
// Shared by dockerfile_runner_test.go and host_runner_extra_test.go.
func writeExecutable(path, body string) error {
	if err := os.WriteFile(path, []byte(body), 0755); err != nil {
		return err
	}
	return os.Chmod(path, 0755)
}

func TestHostRunner_Restart_StartsFreshProcess(t *testing.T) {
	if _, err := exec.LookPath("sleep"); err != nil {
		t.Skip("sleep not available")
	}

	r := &HostRunner{}
	dir := t.TempDir()
	svc := interfaces.ServiceContext{
		Name:        "restarter",
		Path:        dir,
		ProjectName: "host-restart-" + t.Name(),
		Detection: detect.DetectResult{
			Runtime:      detect.RuntimeMake,
			StartCommand: "sleep 30",
		},
	}
	t.Cleanup(func() {
		_ = r.Stop(context.Background(), svc)
		os.RemoveAll(naming.TempDir(svc.ProjectName))
	})

	ctx := context.Background()
	if err := r.Start(ctx, svc); err != nil {
		t.Fatalf("Start: %v", err)
	}
	firstPID := r.GetPID("restarter")
	if firstPID == 0 {
		t.Fatal("expected first PID to be set")
	}

	time.Sleep(100 * time.Millisecond)

	if err := r.Restart(ctx, svc); err != nil {
		t.Errorf("Restart: %v", err)
	}

	secondPID := r.GetPID("restarter")
	if secondPID == 0 {
		t.Error("expected second PID after restart")
	}
	if secondPID == firstPID {
		t.Errorf("expected a different PID after restart, got same: %d", secondPID)
	}
}

func TestHostRunner_Restart_WhenNotRunning(t *testing.T) {
	// Restart should still work when the service was never started (Stop is a no-op).
	if _, err := exec.LookPath("sleep"); err != nil {
		t.Skip("sleep not available")
	}

	r := &HostRunner{}
	dir := t.TempDir()
	svc := interfaces.ServiceContext{
		Name:        "cold-restart",
		Path:        dir,
		ProjectName: "host-cold-" + t.Name(),
		Detection: detect.DetectResult{
			Runtime:      detect.RuntimeMake,
			StartCommand: "sleep 30",
		},
	}
	t.Cleanup(func() {
		_ = r.Stop(context.Background(), svc)
		os.RemoveAll(naming.TempDir(svc.ProjectName))
	})

	if err := r.Restart(context.Background(), svc); err != nil {
		t.Errorf("Restart from cold state: %v", err)
	}
	if r.GetPID("cold-restart") == 0 {
		t.Error("expected PID after cold restart")
	}
}

func TestHostRunner_Logs_ReadsFile(t *testing.T) {
	if _, err := exec.LookPath("tail"); err != nil {
		t.Skip("tail not available")
	}

	r := &HostRunner{}
	svc := interfaces.ServiceContext{
		Name:        "api",
		ProjectName: "host-logs-" + t.Name(),
	}
	t.Cleanup(func() { os.RemoveAll(naming.TempDir(svc.ProjectName)) })

	// Ensure log directory/file exist.
	logDir := naming.LogDir(svc.ProjectName)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	logFile := naming.LogFile(svc.ProjectName, svc.Name)
	if err := os.WriteFile(logFile, []byte("line1\nline2\n"), 0644); err != nil {
		t.Fatalf("write log: %v", err)
	}

	// follow=false, tail=1 exercises the tail flag path.
	if err := r.Logs(context.Background(), svc, false, 1); err != nil {
		t.Errorf("Logs: %v", err)
	}
}

func TestHostRunner_Logs_NoTailNoFollow(t *testing.T) {
	if _, err := exec.LookPath("tail"); err != nil {
		t.Skip("tail not available")
	}

	r := &HostRunner{}
	svc := interfaces.ServiceContext{
		Name:        "api",
		ProjectName: "host-logs-notail-" + t.Name(),
	}
	t.Cleanup(func() { os.RemoveAll(naming.TempDir(svc.ProjectName)) })

	logDir := naming.LogDir(svc.ProjectName)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	logFile := naming.LogFile(svc.ProjectName, svc.Name)
	if err := os.WriteFile(logFile, []byte("hello\n"), 0644); err != nil {
		t.Fatalf("write log: %v", err)
	}

	// tail=0, follow=false: no flags beyond the file path.
	if err := r.Logs(context.Background(), svc, false, 0); err != nil {
		t.Errorf("Logs: %v", err)
	}
}

func TestHostRunner_Logs_FollowTerminatedByContext(t *testing.T) {
	if _, err := exec.LookPath("tail"); err != nil {
		t.Skip("tail not available")
	}

	r := &HostRunner{}
	svc := interfaces.ServiceContext{
		Name:        "api",
		ProjectName: "host-logs-follow-" + t.Name(),
	}
	t.Cleanup(func() { os.RemoveAll(naming.TempDir(svc.ProjectName)) })

	logDir := naming.LogDir(svc.ProjectName)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	logFile := naming.LogFile(svc.ProjectName, svc.Name)
	if err := os.WriteFile(logFile, []byte(""), 0644); err != nil {
		t.Fatalf("write log: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	// follow=true will block on tail -f; ctx timeout kills the child.
	// We don't care whether Logs returns nil or an error, just that it returns.
	_ = r.Logs(ctx, svc, true, 0)
}

func TestHostRunner_Stop_CustomStopCommand(t *testing.T) {
	// Exercises the svc.StopCommand branch — "true" exits 0.
	if _, err := exec.LookPath("true"); err != nil {
		t.Skip("true not available")
	}

	r := &HostRunner{}
	// Pre-populate a PID so we can verify it gets cleared after custom stop.
	r.SetPID("api", 999999)
	svc := interfaces.ServiceContext{
		Name:        "api",
		Path:        t.TempDir(),
		StopCommand: "true",
		EnvVars:     map[string]string{"CUSTOM": "yes"},
	}

	if err := r.Stop(context.Background(), svc); err != nil {
		t.Errorf("Stop: %v", err)
	}
	if pid := r.GetPID("api"); pid != 0 {
		t.Errorf("expected PID cleared, got %d", pid)
	}
}

func TestHostRunner_Stop_CustomStopCommandFails(t *testing.T) {
	if _, err := exec.LookPath("false"); err != nil {
		t.Skip("false not available")
	}

	r := &HostRunner{}
	svc := interfaces.ServiceContext{
		Name:        "api",
		Path:        t.TempDir(),
		StopCommand: "false",
	}

	if err := r.Stop(context.Background(), svc); err == nil {
		t.Error("expected error when stop command exits non-zero")
	}
}

func TestHostRunner_Stop_CustomStopNoPriorPID(t *testing.T) {
	// Custom stop path with no pre-existing processes map should be safe.
	if _, err := exec.LookPath("true"); err != nil {
		t.Skip("true not available")
	}

	r := &HostRunner{} // processes is nil
	svc := interfaces.ServiceContext{
		Name:        "api",
		Path:        t.TempDir(),
		StopCommand: "true",
	}

	if err := r.Stop(context.Background(), svc); err != nil {
		t.Errorf("Stop: %v", err)
	}
}

func TestHostRunner_Stop_PollingLoopExercised(t *testing.T) {
	// Start a long-sleeping process, then Stop(); this exercises the
	// SIGTERM → poll → delete path in Stop() rather than the early ESRCH
	// short-circuit. Use a short sleep so the test is cheap.
	if _, err := exec.LookPath("sleep"); err != nil {
		t.Skip("sleep not available")
	}

	r := &HostRunner{}
	dir := t.TempDir()
	svc := interfaces.ServiceContext{
		Name:        "poll-sleeper",
		Path:        dir,
		ProjectName: "host-poll-" + t.Name(),
		Detection: detect.DetectResult{
			Runtime:      detect.RuntimeMake,
			StartCommand: "sleep 10",
		},
	}
	t.Cleanup(func() { os.RemoveAll(naming.TempDir(svc.ProjectName)) })

	if err := r.Start(context.Background(), svc); err != nil {
		t.Fatalf("Start: %v", err)
	}
	// Let the process settle so SIGTERM actually finds the group.
	time.Sleep(100 * time.Millisecond)

	start := time.Now()
	if err := r.Stop(context.Background(), svc); err != nil {
		t.Errorf("Stop: %v", err)
	}
	elapsed := time.Since(start)

	// sleep respects SIGTERM immediately, so stop should be well under 5s.
	if elapsed > 4*time.Second {
		t.Errorf("Stop took too long (%v) — polling loop hit 5s deadline", elapsed)
	}
	if pid := r.GetPID("poll-sleeper"); pid != 0 {
		t.Errorf("expected PID cleared after Stop, got %d", pid)
	}
}

func TestHostRunner_Logs_UsesNamingLogFile(t *testing.T) {
	// Sanity — make sure the log path Logs uses is the naming-derived one.
	svc := interfaces.ServiceContext{
		Name:        "api",
		ProjectName: "host-logsfile-" + t.Name(),
	}
	t.Cleanup(func() { os.RemoveAll(naming.TempDir(svc.ProjectName)) })

	expected := naming.LogFile(svc.ProjectName, svc.Name)
	if filepath.Base(expected) != "api.log" {
		t.Errorf("unexpected log basename: %s", filepath.Base(expected))
	}
}
