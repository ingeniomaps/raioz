package upcase

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"raioz/internal/domain/models"
	"raioz/internal/host"
	"raioz/internal/state"
)

// startIn launches a long-lived process whose cwd is dir, the way a host
// service runs, and returns its PID.
func startIn(t *testing.T, dir string) int {
	t.Helper()
	cmd := exec.Command("sleep", "30")
	cmd.Dir = dir
	host.SetNewProcessGroup(cmd)
	if err := cmd.Start(); err != nil {
		t.Skipf("sleep not available: %v", err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _, _ = cmd.Process.Wait() })
	return cmd.Process.Pid
}

// up never stops a running host service: it reports the ones still up so
// the start step adopts them, and forgets recorded PIDs that no longer
// stand for the service.
func TestRunningHostServices(t *testing.T) {
	projectDir := t.TempDir()
	webDir := filepath.Join(projectDir, "web")
	apiDir := filepath.Join(projectDir, "api")
	for _, d := range []string{webDir, apiDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	deps := &models.Deps{Services: map[string]models.Service{
		"web":   {Source: models.SourceConfig{Kind: "local", Path: webDir}},
		"api":   {Source: models.SourceConfig{Kind: "local", Path: apiDir}},
		"dead":  {Source: models.SourceConfig{Kind: "local", Path: apiDir}},
		"other": {Source: models.SourceConfig{Kind: "local", Path: apiDir}},
	}}

	webPID := startIn(t, webDir)
	// A live process that runs somewhere else: the PID api once had was
	// handed to something unrelated.
	strayPID := startIn(t, t.TempDir())

	save := func() {
		t.Helper()
		if err := state.SaveLocalState(projectDir, &models.LocalState{
			Project: "p",
			HostPIDs: map[string]int{
				"web": webPID, "api": strayPID, "dead": 999999991, "other": 999999992,
			},
		}); err != nil {
			t.Fatal(err)
		}
	}
	scope := map[string]struct{}{"web": {}, "api": {}, "dead": {}}

	save()
	running := runningHostServices(context.Background(), projectDir, deps, scope)

	if running["web"] != webPID {
		t.Errorf("running = %v, want web:%d adopted", running, webPID)
	}
	if !host.IsProcessAlive(webPID) || !host.IsProcessAlive(strayPID) {
		t.Error("a process was killed; up must not stop anything")
	}
	if _, ok := running["dead"]; ok {
		t.Errorf("dead PID reported as running: %v", running)
	}
	if runtime.GOOS == "linux" {
		if _, ok := running["api"]; ok {
			t.Errorf("a PID running outside the service path was adopted: %v", running)
		}
	}

	loaded, err := state.LoadLocalState(projectDir)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.HostPIDs["web"] != webPID {
		t.Errorf("state lost the running service: %v", loaded.HostPIDs)
	}
	if _, ok := loaded.HostPIDs["dead"]; ok {
		t.Errorf("dead PID kept in state: %v", loaded.HostPIDs)
	}
	if _, ok := loaded.HostPIDs["other"]; !ok {
		t.Errorf("out-of-scope PID must survive a selective up: %v", loaded.HostPIDs)
	}

	t.Run("empty scope touches nothing", func(t *testing.T) {
		save()
		if got := runningHostServices(context.Background(), projectDir, deps, nil); got != nil {
			t.Errorf("got %v, want nil", got)
		}
		loaded, _ := state.LoadLocalState(projectDir)
		if len(loaded.HostPIDs) != 4 {
			t.Errorf("state changed: %v", loaded.HostPIDs)
		}
	})

	t.Run("no state file", func(t *testing.T) {
		if got := runningHostServices(context.Background(), t.TempDir(), deps, scope); got != nil {
			t.Errorf("got %v, want nil", got)
		}
	})
}
