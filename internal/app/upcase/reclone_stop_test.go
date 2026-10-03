package upcase

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"raioz/internal/domain/models"
	"raioz/internal/state"
)

func TestStopGitServicesBeforeReclone(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("matches the process to its service through /proc")
	}
	projectDir := t.TempDir()
	gitDir := filepath.Join(projectDir, "ext")
	localDir := filepath.Join(projectDir, "web")
	for _, d := range []string{gitDir, localDir} {
		if err := os.Mkdir(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	start := func(dir string) int {
		cmd := exec.Command("sleep", "30")
		cmd.Dir = dir
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = cmd.Process.Kill(); _, _ = cmd.Process.Wait() })
		return cmd.Process.Pid
	}
	gitPID, localPID := start(gitDir), start(localDir)

	deps := &models.Deps{Services: map[string]models.Service{
		"ext": {Source: models.SourceConfig{Kind: "git", Path: gitDir}},
		"web": {Source: models.SourceConfig{Kind: "local", Path: localDir}},
	}}
	if err := state.SaveLocalState(projectDir, &models.LocalState{
		Project: "p", HostPIDs: map[string]int{"ext": gitPID, "web": localPID},
	}); err != nil {
		t.Fatal(err)
	}

	var stoppedPIDs []int
	prev := stopHostServiceFn
	stopHostServiceFn = func(_ context.Context, pid int, _ string, _ string) error {
		stoppedPIDs = append(stoppedPIDs, pid)
		return nil
	}
	t.Cleanup(func() { stopHostServiceFn = prev })

	stopGitServicesBeforeReclone(context.Background(), deps, projectDir)

	if len(stoppedPIDs) != 1 || stoppedPIDs[0] != gitPID {
		t.Fatalf("only the git service must be stopped, got %v (git pid %d)", stoppedPIDs, gitPID)
	}
	got, err := state.LoadLocalState(projectDir)
	if err != nil {
		t.Fatal(err)
	}
	if _, still := got.HostPIDs["ext"]; still {
		t.Error("the stopped service must leave the recorded PIDs")
	}
	if got.HostPIDs["web"] != localPID {
		t.Error("a service that is not re-cloned keeps its PID")
	}
}
