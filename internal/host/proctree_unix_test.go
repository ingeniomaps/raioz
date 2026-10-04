//go:build !windows

package host

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestParentPID(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("parentPID reads /proc; linux only")
	}
	ppid, ok := parentPID(os.Getpid())
	if !ok {
		t.Fatal("parentPID(self) failed")
	}
	if ppid != os.Getppid() {
		t.Errorf("parentPID(self) = %d, want %d", ppid, os.Getppid())
	}
	if _, ok := parentPID(-1); ok {
		t.Error("parentPID(-1) should fail")
	}
}

func TestAncestorPIDs(t *testing.T) {
	protected := ancestorPIDs()
	if !protected[os.Getpid()] {
		t.Error("ancestor set must contain self")
	}
	if runtime.GOOS == "linux" && !protected[os.Getppid()] {
		t.Error("ancestor set must contain the parent process")
	}
	if len(protected) > maxAncestorWalk+1 {
		t.Errorf("ancestor set has %d entries, walk cap is %d", len(protected), maxAncestorWalk)
	}
}

// deepSweepDir returns a directory deep enough to pass minCwdComponents.
func deepSweepDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "svc", "app")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestKillOrphansByCwd_KillsProcessRootedInPath(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("cwd sweep walks /proc; linux only")
	}
	dir := deepSweepDir(t)

	cmd := exec.Command("sleep", "30")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), ServiceMarkerEnv+"=proj/svc")
	SetNewProcessGroup(cmd)
	if err := cmd.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	pid := cmd.Process.Pid
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	killed := KillOrphansByCwd(dir, "proj/svc")
	if !slices.Contains(killed, pid) {
		_ = ForceKillProcessTree(pid)
		<-done
		t.Fatalf("sweep of %s killed %v, want it to include %d", dir, killed, pid)
	}
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		_ = ForceKillProcessTree(pid)
		<-done
		t.Error("swept process didn't exit within 3s")
	}
}

// TestKillOrphansByCwd_SparesAncestors locks the issue-022 regression: the
// sweep must never signal the chain that invoked it. The test process
// chdirs INTO the swept dir (playing the dev's shell running `raioz down`
// from inside a `path: .` project) and re-execs itself as a child that
// performs the sweep. Pre-fix, the child SIGTERMed this process and the
// whole test binary died; post-fix the parent is in the child's ancestor
// chain and survives.
func TestKillOrphansByCwd_SparesAncestors(t *testing.T) {
	if os.Getenv("RAIOZ_TEST_SWEEP_HELPER") == "1" {
		killed := KillOrphansByCwd(os.Getenv("RAIOZ_TEST_SWEEP_DIR"), "proj/svc")
		fmt.Printf("SWEPT=%v\n", killed)
		return
	}
	if runtime.GOOS != "linux" {
		t.Skip("cwd sweep walks /proc; linux only")
	}
	dir := deepSweepDir(t)
	t.Chdir(dir)

	cmd := exec.Command(os.Args[0], "-test.run", "^TestKillOrphansByCwd_SparesAncestors$")
	cmd.Dir = os.TempDir() // child's own cwd stays outside the swept path
	cmd.Env = append(os.Environ(),
		"RAIOZ_TEST_SWEEP_HELPER=1", "RAIOZ_TEST_SWEEP_DIR="+dir)
	// The invoker carries the marker too — a terminal opened from inside
	// the service — which is the case the ancestor exclusion is for.
	t.Setenv(ServiceMarkerEnv, "proj/svc")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("helper failed: %v\n%s", err, out)
	}
	swept, ok := parseSweptPIDs(string(out))
	if !ok {
		t.Fatalf("helper produced no sweep marker:\n%s", out)
	}
	if slices.Contains(swept, os.Getpid()) {
		t.Errorf("sweep signalled an ancestor (pid %d), swept=%v", os.Getpid(), swept)
	}
}

// parseSweptPIDs extracts the pid list from the helper's "SWEPT=[...]" line.
func parseSweptPIDs(out string) ([]int, bool) {
	for line := range strings.SplitSeq(out, "\n") {
		rest, found := strings.CutPrefix(line, "SWEPT=")
		if !found {
			continue
		}
		rest = strings.Trim(rest, "[]")
		var pids []int
		for f := range strings.FieldsSeq(rest) {
			pid, err := strconv.Atoi(f)
			if err != nil {
				return nil, false
			}
			pids = append(pids, pid)
		}
		return pids, true
	}
	return nil, false
}

// The service directory is also where the user's editor, shells and tools
// run. Only what raioz started as that service is swept: same directory
// without the marker, or another service's marker, is left alone.
func TestKillOrphansByCwd_SparesUnmarkedProcesses(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("cwd sweep walks /proc; linux only")
	}
	dir := deepSweepDir(t)

	start := func(env ...string) (*exec.Cmd, chan error) {
		cmd := exec.Command("sleep", "30")
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), env...)
		SetNewProcessGroup(cmd)
		if err := cmd.Start(); err != nil {
			t.Fatalf("Start: %v", err)
		}
		done := make(chan error, 1)
		go func() { done <- cmd.Wait() }()
		t.Cleanup(func() { _ = ForceKillProcessTree(cmd.Process.Pid); <-done })
		return cmd, done
	}
	editor, _ := start()
	other, _ := start(ServiceMarkerEnv + "=proj/other")

	killed := KillOrphansByCwd(dir, "proj/svc")
	if len(killed) != 0 {
		t.Errorf("nothing carries the marker, yet swept %v", killed)
	}
	for name, cmd := range map[string]*exec.Cmd{"unmarked": editor, "other service": other} {
		if !IsProcessAlive(cmd.Process.Pid) {
			t.Errorf("%s process in the service dir was killed", name)
		}
	}
	if got := KillOrphansByCwd(dir, ""); got != nil {
		t.Errorf("an empty marker must sweep nothing, got %v", got)
	}
}

// StopProcessTree returns only when the whole group is gone: the leader
// exits on SIGTERM at once while a child that ignores it holds on, and a
// relaunch needs the child's port.
func TestStopProcessTree_WaitsForTheWholeGroup(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "child.pid")
	// The child ignores SIGTERM; the leader dies with it.
	script := `(trap "" TERM; echo $BASHPID > ` + marker + `; sleep 60) & wait`
	cmd := exec.Command("bash", "-c", script)
	SetNewProcessGroup(cmd)
	if err := cmd.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	go func() { _ = cmd.Wait() }()

	var child int
	for range 100 {
		if data, err := os.ReadFile(marker); err == nil {
			if child, _ = strconv.Atoi(strings.TrimSpace(string(data))); child > 0 {
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	if child == 0 {
		_ = ForceKillProcessTree(cmd.Process.Pid)
		t.Fatal("child never reported its pid")
	}

	if err := StopProcessTree(context.Background(), cmd.Process.Pid); err != nil {
		t.Fatalf("StopProcessTree: %v", err)
	}
	if IsProcessAlive(child) {
		_ = syscall.Kill(child, syscall.SIGKILL)
		t.Error("StopProcessTree returned while a process of the group was still alive")
	}
	if IsProcessGroupAlive(cmd.Process.Pid) {
		t.Error("the group is reported alive after StopProcessTree")
	}
}
