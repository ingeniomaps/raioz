//go:build !windows

package host

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
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
	SetNewProcessGroup(cmd)
	if err := cmd.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	pid := cmd.Process.Pid
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	killed := KillOrphansByCwd(dir)
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
		killed := KillOrphansByCwd(os.Getenv("RAIOZ_TEST_SWEEP_DIR"))
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
