package host

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"raioz/internal/workspace"
)

const hostProcessesFileName = ".host-processes.json"

// HostProcessesState contains the state of host processes
type HostProcessesState struct {
	Processes map[string]*ProcessInfo `json:"processes"` // service name -> process info
}

// SaveProcessesState saves the host processes state to disk
func SaveProcessesState(ws *workspace.Workspace, processes map[string]*ProcessInfo) error {
	state := HostProcessesState{
		Processes: processes,
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal host processes state: %w", err)
	}

	path := filepath.Join(ws.Root, hostProcessesFileName)
	// Use 0600 permissions (read/write for owner only) for security
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write host processes state: %w", err)
	}

	return nil
}

// LoadProcessesState loads the host processes state from disk
func LoadProcessesState(ws *workspace.Workspace) (map[string]*ProcessInfo, error) {
	path := filepath.Join(ws.Root, hostProcessesFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]*ProcessInfo), nil // No processes running
		}
		return nil, fmt.Errorf("failed to read host processes state: %w", err)
	}

	var state HostProcessesState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to unmarshal host processes state: %w", err)
	}

	if state.Processes == nil {
		return make(map[string]*ProcessInfo), nil
	}

	return state.Processes, nil
}

// RemoveProcessesState removes the host processes state file
func RemoveProcessesState(ws *workspace.Workspace) error {
	path := filepath.Join(ws.Root, hostProcessesFileName)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove host processes state: %w", err)
	}
	return nil
}
