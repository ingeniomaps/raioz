package host

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"raioz/internal/workspace"
)

func newTestWorkspace(t *testing.T) *workspace.Workspace {
	t.Helper()
	return &workspace.Workspace{Root: t.TempDir()}
}

func TestLoadProcessesStateMissingFile(t *testing.T) {
	ws := newTestWorkspace(t)

	got, err := LoadProcessesState(ws)
	if err != nil {
		t.Errorf("LoadProcessesState() error = %v, want nil", err)
	}
	if got == nil {
		t.Errorf("LoadProcessesState() = nil, want empty map")
	}
	if len(got) != 0 {
		t.Errorf("LoadProcessesState() len = %d, want 0", len(got))
	}
}

func TestSaveAndLoadProcessesState(t *testing.T) {
	ws := newTestWorkspace(t)

	processes := map[string]*ProcessInfo{
		"api": {
			PID:         1234,
			Service:     "api",
			Command:     "npm run dev",
			StopCommand: "npm stop",
			ComposePath: "/tmp/docker-compose.yml",
			StartTime:   time.Now().Truncate(time.Second),
		},
		"worker": {
			PID:       5678,
			Service:   "worker",
			Command:   "go run main.go",
			StartTime: time.Now().Truncate(time.Second),
		},
	}

	if err := SaveProcessesState(ws, processes); err != nil {
		t.Fatalf("SaveProcessesState() error = %v, want nil", err)
	}

	// Verify file exists with correct permissions
	path := filepath.Join(ws.Root, ".host-processes.json")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat state file: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("state file permissions = %v, want 0600", info.Mode().Perm())
	}

	// Load and verify round-trip
	loaded, err := LoadProcessesState(ws)
	if err != nil {
		t.Fatalf("LoadProcessesState() error = %v", err)
	}
	if len(loaded) != 2 {
		t.Errorf("loaded len = %d, want 2", len(loaded))
	}
	if loaded["api"] == nil || loaded["api"].PID != 1234 {
		t.Errorf("loaded[api].PID = %v, want 1234", loaded["api"])
	}
	if loaded["api"].Command != "npm run dev" {
		t.Errorf("loaded[api].Command = %q, want %q", loaded["api"].Command, "npm run dev")
	}
	if loaded["worker"] == nil || loaded["worker"].PID != 5678 {
		t.Errorf("loaded[worker].PID = %v, want 5678", loaded["worker"])
	}
}

func TestSaveProcessesStateEmpty(t *testing.T) {
	ws := newTestWorkspace(t)

	if err := SaveProcessesState(ws, map[string]*ProcessInfo{}); err != nil {
		t.Errorf("SaveProcessesState() error = %v, want nil", err)
	}

	loaded, err := LoadProcessesState(ws)
	if err != nil {
		t.Errorf("LoadProcessesState() error = %v, want nil", err)
	}
	if len(loaded) != 0 {
		t.Errorf("loaded len = %d, want 0", len(loaded))
	}
}

func TestLoadProcessesStateInvalidJSON(t *testing.T) {
	ws := newTestWorkspace(t)

	path := filepath.Join(ws.Root, ".host-processes.json")
	if err := os.WriteFile(path, []byte("not json"), 0600); err != nil {
		t.Fatalf("write bad file: %v", err)
	}

	_, err := LoadProcessesState(ws)
	if err == nil {
		t.Errorf("LoadProcessesState() error = nil, want unmarshal error")
	}
}

func TestLoadProcessesStateNilProcesses(t *testing.T) {
	ws := newTestWorkspace(t)

	// Write JSON with no processes field
	path := filepath.Join(ws.Root, ".host-processes.json")
	data, _ := json.Marshal(map[string]interface{}{})
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatalf("write: %v", err)
	}

	loaded, err := LoadProcessesState(ws)
	if err != nil {
		t.Errorf("LoadProcessesState() error = %v", err)
	}
	if loaded == nil {
		t.Errorf("loaded = nil, want empty map")
	}
	if len(loaded) != 0 {
		t.Errorf("loaded len = %d, want 0", len(loaded))
	}
}

func TestRemoveProcessesState(t *testing.T) {
	ws := newTestWorkspace(t)

	// Save first
	if err := SaveProcessesState(ws, map[string]*ProcessInfo{"a": {PID: 1}}); err != nil {
		t.Fatalf("save: %v", err)
	}

	if err := RemoveProcessesState(ws); err != nil {
		t.Errorf("RemoveProcessesState() error = %v, want nil", err)
	}

	path := filepath.Join(ws.Root, ".host-processes.json")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("file still exists after remove")
	}
}

func TestRemoveProcessesStateMissingFile(t *testing.T) {
	ws := newTestWorkspace(t)

	// Should not error when file is absent
	if err := RemoveProcessesState(ws); err != nil {
		t.Errorf("RemoveProcessesState() error = %v, want nil", err)
	}
}
