package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const projectStateFile = ".raioz.state.json"

// LocalState is the minimal state file stored in the project directory.
// Docker is the source of truth for running state; this file only stores
// what Docker can't tell us: dev overrides, ignored services, host PIDs.
type LocalState struct {
	Project      string                 `json:"project"`
	Workspace    string                 `json:"workspace,omitempty"`
	LastUp       time.Time              `json:"lastUp"`
	DevOverrides map[string]DevOverride `json:"devOverrides,omitempty"`
	Ignored      []string               `json:"ignored,omitempty"`
	HostPIDs     map[string]int         `json:"hostPIDs,omitempty"`
	NetworkName  string                 `json:"networkName,omitempty"`
}

// DevOverride records that a dependency has been promoted to local development.
type DevOverride struct {
	OriginalImage string `json:"originalImage"`
	LocalPath     string `json:"localPath"`
	PromotedAt    time.Time `json:"promotedAt"`
}

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
		return err
	}
	return nil
}

// AddDevOverride records a dependency being promoted to local.
func (s *LocalState) AddDevOverride(name, originalImage, localPath string) {
	s.DevOverrides[name] = DevOverride{
		OriginalImage: originalImage,
		LocalPath:     localPath,
		PromotedAt:    time.Now(),
	}
}

// RemoveDevOverride removes a dev override.
func (s *LocalState) RemoveDevOverride(name string) {
	delete(s.DevOverrides, name)
}

// IsDevOverridden returns true if a dependency is currently in dev mode.
func (s *LocalState) IsDevOverridden(name string) bool {
	_, ok := s.DevOverrides[name]
	return ok
}

// GetDevOverride returns the dev override for a dependency, if any.
func (s *LocalState) GetDevOverride(name string) (DevOverride, bool) {
	o, ok := s.DevOverrides[name]
	return o, ok
}
