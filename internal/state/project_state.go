package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"raioz/internal/domain/models"
)

const projectStateFile = ".raioz.state.json"

// LocalState / DevOverride live canonically in internal/domain/models;
// the aliases keep `models.LocalState` etc. callers compiling (ADR-009).
type (
	LocalState  = models.LocalState
	DevOverride = models.DevOverride
)

// LoadLocalState loads the project state from the project directory.
// Returns an empty state if the file doesn't exist.
func LoadLocalState(projectDir string) (*LocalState, error) {
	path := filepath.Join(projectDir, projectStateFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &LocalState{
				DevOverrides: make(map[string]DevOverride),
				HostPIDs:     make(map[string]int),
			}, nil
		}
		return nil, fmt.Errorf("failed to read project state: %w", err)
	}

	var state LocalState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to parse project state: %w", err)
	}

	if state.DevOverrides == nil {
		state.DevOverrides = make(map[string]DevOverride)
	}
	if state.HostPIDs == nil {
		state.HostPIDs = make(map[string]int)
	}

	return &state, nil
}

// SaveLocalState saves the project state to the project directory.
func SaveLocalState(projectDir string, state *LocalState) error {
	path := filepath.Join(projectDir, projectStateFile)
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal project state: %w", err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write project state: %w", err)
	}
	return nil
}

// RemoveLocalState removes the state file from the project directory.
func RemoveLocalState(projectDir string) error {
	path := filepath.Join(projectDir, projectStateFile)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove state file %q: %w", path, err)
	}
	return nil
}
