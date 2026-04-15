package upcase

import (
	"testing"

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
	saveHostPIDs(dir, "proj", "acme", "acme-net", nil, []string{}, nil)

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
	saveHostPIDs(dir, "proj", "", "net", nil, []string{"api"}, detections)
}

// --- cleanStaleHostProcesses with stale alive PID ----------------------------

func TestCleanStaleHostProcessesWithHighPID(t *testing.T) {
	dir := t.TempDir()
	ls := &state.LocalState{
		Project:  "p",
		HostPIDs: map[string]int{"svc": 999999999},
	}
	if err := state.SaveLocalState(dir, ls); err != nil {
		t.Fatal(err)
	}
	// Dead PID → should just skip, no crash
	cleanStaleHostProcesses(nil, dir, "p")
}

// --- isProcessAlive with PID 0 -----------------------------------------------

func TestIsProcessAliveZero(t *testing.T) {
	// PID 0 is special on Unix (process group leader).
	// On Linux FindProcess(0) succeeds but signal(0) may fail.
	// Just verify no panic.
	_ = isProcessAlive(0)
}
