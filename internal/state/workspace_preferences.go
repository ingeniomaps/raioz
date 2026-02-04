package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"raioz/internal/workspace"
)

const workspacePreferencesFileName = "workspace-preferences.json"

// WorkspaceProjectPreference stores which project to use when multiple .raioz.json
// in the same workspace define overlapping services (e.g. same service name).
type WorkspaceProjectPreference struct {
	PreferredProject   string `json:"preferredProject"`   // project name to use when conflict
	AlwaysAsk          bool   `json:"alwaysAsk"`          // if true, always prompt instead of applying preference
	MergeWhenPreferred bool   `json:"mergeWhenPreferred"` // if true and preferred project matches, merge configs instead of replace
}

// WorkspacePreferences is the file format: workspace name -> preference
type WorkspacePreferences struct {
	ByWorkspace map[string]WorkspaceProjectPreference `json:"byWorkspace"`
}

func getWorkspacePreferencesPath() (string, error) {
	base, err := workspace.GetBaseDir()
	if err != nil {
		return "", fmt.Errorf("failed to get base directory: %w", err)
	}
	return filepath.Join(base, workspacePreferencesFileName), nil
}

func loadWorkspacePreferences() (*WorkspacePreferences, error) {
	path, err := getWorkspacePreferencesPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &WorkspacePreferences{
				ByWorkspace: make(map[string]WorkspaceProjectPreference),
			}, nil
		}
		return nil, fmt.Errorf("failed to read workspace preferences: %w", err)
	}
	var prefs WorkspacePreferences
	if err := json.Unmarshal(data, &prefs); err != nil {
		return nil, fmt.Errorf("failed to unmarshal workspace preferences: %w", err)
	}
	if prefs.ByWorkspace == nil {
		prefs.ByWorkspace = make(map[string]WorkspaceProjectPreference)
	}
	return &prefs, nil
}

func saveWorkspacePreferences(prefs *WorkspacePreferences) error {
	path, err := getWorkspacePreferencesPath()
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create preferences directory: %w", err)
	}
	data, err := json.MarshalIndent(prefs, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal workspace preferences: %w", err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write workspace preferences: %w", err)
	}
	return nil
}

// GetWorkspaceProjectPreference returns the stored preference for a workspace, or nil if not set
func GetWorkspaceProjectPreference(workspaceName string) (*WorkspaceProjectPreference, error) {
	prefs, err := loadWorkspacePreferences()
	if err != nil {
		return nil, err
	}
	p, ok := prefs.ByWorkspace[workspaceName]
	if !ok {
		return nil, nil
	}
	return &p, nil
}

// SetWorkspaceProjectPreference saves the preference for a workspace
func SetWorkspaceProjectPreference(workspaceName string, pref WorkspaceProjectPreference) error {
	prefs, err := loadWorkspacePreferences()
	if err != nil {
		return err
	}
	if prefs.ByWorkspace == nil {
		prefs.ByWorkspace = make(map[string]WorkspaceProjectPreference)
	}
	prefs.ByWorkspace[workspaceName] = pref
	return saveWorkspacePreferences(prefs)
}
