package upcase

import (
	"context"
	"os"
	"os/exec"
	"testing"
	"time"

	"raioz/internal/domain/interfaces"
	"raioz/internal/domain/models"
	"raioz/internal/naming"
	"raioz/internal/orchestrate"
	"raioz/internal/state"
)

// --- killProcessGraceful -----------------------------------------------------

func TestKillProcessGracefulInvalidPID(t *testing.T) {
	// Negative PIDs are silently ignored (kill(-1) would signal ALL user processes)
	killProcessGraceful(-1)
	killProcessGraceful(0)
}

func TestKillProcessGracefulNonexistentPID(t *testing.T) {
	// Use a very high PID unlikely to exist
	killProcessGraceful(999999999)
}

// --- isProcessRunning --------------------------------------------------------

func TestIsProcessRunningZero(t *testing.T) {
	if isProcessRunning(0) {
		t.Error("PID 0 should not be running")
	}
}

// --- saveHostPIDs with service names -----------------------------------------

func TestSaveHostPIDsWithServiceNames(t *testing.T) {
	dir := t.TempDir()

	// saveHostPIDs needs a dispatcher which is hard to mock.
	// But we can test the early short-circuit with empty serviceNames.
	saveHostPIDs(dir, "proj", "acme", "acme-net", nil, []string{}, nil, nil)

	// State file must exist with project/workspace/network populated even
	// when no host PIDs were started.
	ls, loadErr := state.LoadLocalState(dir)
	if loadErr != nil {
		t.Fatal(loadErr)
	}
	if ls.Project != "proj" {
		t.Errorf("Project = %q, want proj", ls.Project)
	}
	if ls.Workspace != "acme" {
		t.Errorf("Workspace = %q, want acme", ls.Workspace)
	}
	if ls.NetworkName != "acme-net" {
		t.Errorf("NetworkName = %q, want acme-net", ls.NetworkName)
	}
	if ls.LastUp.IsZero() {
		t.Error("LastUp should be set")
	}
	if len(ls.HostPIDs) > 0 {
		t.Error("expected no host PIDs saved")
	}
}

func TestSaveHostPIDsCreatesNewState(t *testing.T) {
	dir := t.TempDir()

	// Call with nil dispatcher + empty detections → should not crash
	detections := DetectionMap{}
	saveHostPIDs(dir, "proj", "", "net", nil, []string{"api"}, detections, nil)
}

// --- cleanStaleHostProcesses with stale alive PID ----------------------------

func TestCleanStaleHostProcessesWithHighPID(t *testing.T) {
	dir := t.TempDir()
	ls := &models.LocalState{
		Project:  "p",
		HostPIDs: map[string]int{"svc": 999999999},
	}
	if err := state.SaveLocalState(dir, ls); err != nil {
		t.Fatal(err)
	}
	// Dead PID → should just skip, no crash
	cleanStaleHostProcesses(context.Background(), dir, "p", map[string]struct{}{"svc": {}})
}

// --- isProcessAlive with PID 0 -----------------------------------------------

func TestIsProcessAliveZero(t *testing.T) {
	// PID 0 is special on Unix (process group leader).
	// On Linux FindProcess(0) succeeds but signal(0) may fail.
	// Just verify no panic.
	_ = isProcessAlive(0)
}

// --- savePartialHostPIDs (failed up) -----------------------------------------

// A failed `up` must hand over the PIDs of whatever it already started, or
// those processes become invisible: `down` iterates the persisted HostPIDs
// and `status` falls back to them.
func TestSavePartialHostPIDsRecordsStartedService(t *testing.T) {
	if _, err := exec.LookPath("sleep"); err != nil {
		t.Skip("sleep not available")
	}

	dir := t.TempDir()
	// A previous successful up, long enough ago that the stale sweep would
	// normally run again.
	previousUp := time.Now().Add(-2 * time.Hour)
	if err := state.SaveLocalState(dir, &models.LocalState{
		Project: "proj",
		LastUp:  previousUp,
	}); err != nil {
		t.Fatal(err)
	}

	dispatcher := orchestrate.NewDispatcher(nil)
	svcCtx := interfaces.ServiceContext{
		Name:        "api",
		Path:        t.TempDir(),
		ProjectName: "partial-up-" + t.Name(),
		Detection: models.DetectResult{
			Runtime:      models.RuntimeMake,
			StartCommand: "sleep 5",
		},
	}
	if err := dispatcher.Start(context.Background(), svcCtx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() {
		_ = dispatcher.Stop(context.Background(), svcCtx)
		_ = os.RemoveAll(naming.TempDir(svcCtx.ProjectName))
	})

	detections := DetectionMap{"api": models.DetectResult{Runtime: models.RuntimeMake}}
	savePartialHostPIDs(dir, "proj", "acme", "acme-net", dispatcher,
		[]string{"api"}, detections, nil)

	ls, err := state.LoadLocalState(dir)
	if err != nil || ls == nil {
		t.Fatalf("LoadLocalState: %v", err)
	}
	if ls.HostPIDs["api"] <= 0 {
		t.Errorf("PID of the started service must be persisted, got %v", ls.HostPIDs)
	}
	if ls.NetworkName != "acme-net" {
		t.Errorf("NetworkName = %q, want acme-net", ls.NetworkName)
	}
	// The retry after a failed up runs within the launcher window, and
	// cleanStaleHostProcesses skips its sweep while LastUp is fresh. A
	// bumped LastUp here would leave the orphan alive for the retry to
	// collide with.
	if !ls.LastUp.Equal(previousUp) {
		t.Errorf("LastUp = %v, want it untouched at %v", ls.LastUp, previousUp)
	}
}

// Nothing started ⇒ nothing to rescue: the failure path must not create or
// touch the state file (a `up` that died before the first service leaves no
// trace to clean up).
func TestSavePartialHostPIDsNoopWhenNothingStarted(t *testing.T) {
	dir := t.TempDir()

	savePartialHostPIDs(dir, "proj", "acme", "acme-net", nil, nil, nil, nil)

	if ls, err := state.LoadLocalState(dir); err == nil && ls != nil && ls.Project != "" {
		t.Errorf("no state file expected, got %+v", ls)
	}
}
