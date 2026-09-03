//go:build !windows

package host

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"raioz/internal/domain/models"
	"raioz/internal/workspace"
)

// A synchronous command that never returns used to make StartService never
// return either — and `restart` holds the workspace lock for that whole
// time, so every other raioz command failed with "the lock already exists"
// (observed: 42 minutes). The deadline must bound it, and the whole
// process group must go down with it: killing only the direct child is
// what left the real server orphaned and serving.
func TestStartServiceSyncCommandIsBoundedAndKillsTheGroup(t *testing.T) {
	skipIfNoBinary(t, "sh")

	dir := t.TempDir()
	grandchildPIDFile := filepath.Join(dir, "grandchild.pid")

	// A fake `make` on PATH so the command matches the synchronous list
	// without needing a real make. It spawns a grandchild and blocks, the
	// shape of a launcher that never completes.
	fakeMake := filepath.Join(dir, "make")
	body := "#!/bin/sh\nsleep 30 &\necho $! > " + grandchildPIDFile + "\nwait\n"
	if err := os.WriteFile(fakeMake, []byte(body), 0755); err != nil {
		t.Fatalf("write fake make: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("RAIOZ_LAUNCHER_TIMEOUT", "400ms")

	ws := &workspace.Workspace{Root: t.TempDir()}
	deps := &models.Deps{Project: models.Project{Name: "synctimeout"}}
	svc := models.Service{
		Source: models.SourceConfig{Kind: "local", Path: ".", Command: "make launch"},
	}

	start := time.Now()
	_, err := StartService(context.Background(), ws, deps, "svc", svc, dir)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected an error when the synchronous command outlives its deadline")
	}
	if !strings.Contains(err.Error(), "did not finish within") {
		t.Errorf("error should name the timeout, got: %v", err)
	}
	// The command sleeps 30s; anything near that means the bound didn't hold.
	if elapsed > 10*time.Second {
		t.Errorf("StartService took %s, want it bounded by the deadline", elapsed)
	}

	// The grandchild is the process that would have stayed alive serving a
	// port that raioz no longer knows about.
	raw, readErr := os.ReadFile(grandchildPIDFile)
	if readErr != nil {
		t.Fatalf("fake make never recorded its grandchild: %v", readErr)
	}
	pid, convErr := strconv.Atoi(strings.TrimSpace(string(raw)))
	if convErr != nil {
		t.Fatalf("unreadable grandchild pid %q: %v", raw, convErr)
	}
	if waitForExit(pid, 5*time.Second) {
		return
	}
	_ = ForceKillProcessTree(pid)
	t.Errorf("grandchild pid %d survived the cancelled synchronous start", pid)
}

// waitForExit polls until pid is gone or the deadline passes.
func waitForExit(pid int, within time.Duration) bool {
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		if !IsProcessAlive(pid) {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return !IsProcessAlive(pid)
}

// The reported case: a service launched by a script. `bash start-api.sh`
// used to be classified as synchronous purely because of the `.sh`, so
// StartService blocked for the life of the service and `restart` never
// returned. It must go to the background path — return a live PID, fast.
func TestStartServiceLauncherScriptRunsInBackground(t *testing.T) {
	skipIfNoBinary(t, "bash")

	prev := startSettleWindow
	startSettleWindow = 200 * time.Millisecond
	t.Cleanup(func() { startSettleWindow = prev })

	dir := t.TempDir()
	script := filepath.Join(dir, "start-api.sh")
	if err := os.WriteFile(script, []byte("#!/usr/bin/env bash\nsleep 20\n"), 0755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	ws := &workspace.Workspace{Root: t.TempDir()}
	deps := &models.Deps{Project: models.Project{Name: "launcher"}}
	svc := models.Service{
		Source: models.SourceConfig{Kind: "local", Path: ".", Command: "bash " + script},
	}

	start := time.Now()
	info, err := StartService(context.Background(), ws, deps, "svc", svc, dir)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("StartService: %v", err)
	}
	t.Cleanup(func() {
		if info != nil && info.PID > 0 {
			_ = KillProcessTree(info.PID)
		}
	})

	// The script sleeps 20s. Returning near that means we waited for it.
	if elapsed > 5*time.Second {
		t.Errorf("StartService took %s, want it to return without waiting for the service", elapsed)
	}
	if info == nil || info.PID <= 0 {
		t.Fatalf("expected a tracked PID for a background service, got %+v", info)
	}
	if !IsProcessAlive(info.PID) {
		t.Errorf("pid %d is not alive: the service should still be running", info.PID)
	}
}

// The launcher pattern (ADR-025): `make launch` starts a daemon and
// returns. The daemon inherits the output pipe, so Wait can't finish until
// it dies — which for a daemon means never. WaitDelay bounds that, and the
// ErrWaitDelay it produces over a clean exit must read as success: the
// launcher did its job.
func TestStartServiceLauncherDetachesWithoutFailing(t *testing.T) {
	skipIfNoBinary(t, "sh")

	launcherKillGracePrev := launcherKillGrace
	launcherKillGrace = 300 * time.Millisecond
	t.Cleanup(func() { launcherKillGrace = launcherKillGracePrev })

	dir := t.TempDir()
	daemonPIDFile := filepath.Join(dir, "daemon.pid")
	fakeMake := filepath.Join(dir, "make")
	// Spawn a daemon that holds stdout open, then exit 0 — the launcher shape.
	body := "#!/bin/sh\nsleep 10 &\necho $! > " + daemonPIDFile + "\nexit 0\n"
	if err := os.WriteFile(fakeMake, []byte(body), 0755); err != nil {
		t.Fatalf("write fake make: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("RAIOZ_LAUNCHER_TIMEOUT", "10s")

	ws := &workspace.Workspace{Root: t.TempDir()}
	deps := &models.Deps{Project: models.Project{Name: "launcherdetach"}}
	svc := models.Service{
		Source: models.SourceConfig{Kind: "local", Path: ".", Command: "make launch"},
	}

	info, err := StartService(context.Background(), ws, deps, "svc", svc, dir)
	if err != nil {
		t.Fatalf("a launcher that detached cleanly must not error: %v", err)
	}
	if info == nil || info.PID != 0 {
		t.Errorf("synchronous launcher should report no PID, got %+v", info)
	}

	// The daemon it spawned must still be running: raioz bounded its own
	// wait, it did not tear down what the launcher started.
	raw, readErr := os.ReadFile(daemonPIDFile)
	if readErr != nil {
		t.Fatalf("fake make never recorded its daemon: %v", readErr)
	}
	pid, convErr := strconv.Atoi(strings.TrimSpace(string(raw)))
	if convErr != nil {
		t.Fatalf("unreadable daemon pid %q: %v", raw, convErr)
	}
	t.Cleanup(func() { _ = ForceKillProcessTree(pid) })
	if !IsProcessAlive(pid) {
		t.Errorf("daemon pid %d was killed; the launcher's work must survive", pid)
	}
}
