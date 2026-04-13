package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	raiozErrors "raioz/internal/errors"
	"raioz/internal/workspace"
)

const workspacePreferencesFileName = "workspace-preferences.json"

// WorkspaceProjectPreference stores which project to use when multiple .raioz.json
// in the same workspace define overlapping services (e.g. same service name).
type WorkspaceProjectPreference struct {
	PreferredProject string `json:"preferredProject"` // project name to use when conflict
	AlwaysAsk        bool   `json:"alwaysAsk"`        // if true, always prompt instead of applying preference
	// if true and preferred project matches, merge configs
	MergeWhenPreferred bool `json:"mergeWhenPreferred"`
}

// WorkspacePreferences is the file format: workspace name -> preference
type WorkspacePreferences struct {
	ByWorkspace map[string]WorkspaceProjectPreference `json:"byWorkspace"`
}

func getWorkspacePreferencesPath() (string, error) {
	base, err := workspace.GetBaseDir()
	if err != nil {
		return "", raiozErrors.New(
			raiozErrors.ErrCodeStateLoadError,
			"failed to get base directory for workspace preferences",
		).WithError(err).
			WithSuggestion(
				"Ensure the raioz base directory is properly configured",
			)
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
		return nil, raiozErrors.New(raiozErrors.ErrCodeStateLoadError, "failed to read workspace preferences file").
			WithContext("path", path).
			WithError(err).
			WithSuggestion("Check that the workspace preferences file exists and is readable")
	}
	var prefs WorkspacePreferences
	if err := json.Unmarshal(data, &prefs); err != nil {
		return nil, raiozErrors.New(
			raiozErrors.ErrCodeStateLoadError,
			"failed to parse workspace preferences file",
		).WithContext("path", path).
			WithError(err).
			WithSuggestion(
				"The workspace preferences file may be corrupted. " +
					"Try deleting it — preferences will be " +
					"re-created when needed",
			)
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
		return raiozErrors.New(raiozErrors.ErrCodeStateSaveError, "failed to create workspace preferences directory").
			WithContext("directory", dir).
			WithError(err).
			WithSuggestion("Verify file permissions and available disk space")
	}
	data, err := json.MarshalIndent(prefs, "", "  ")
	if err != nil {
		return raiozErrors.New(
			raiozErrors.ErrCodeStateSaveError,
			fmt.Sprintf(
				"failed to marshal workspace preferences: %v", err,
			),
		).WithContext("path", path).
			WithError(err).
			WithSuggestion(
				"The preferences data may be corrupted. " +
					"Try deleting the preferences file and " +
					"setting preferences again",
			)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return raiozErrors.New(raiozErrors.ErrCodeStateSaveError, "failed to write workspace preferences file").
			WithContext("path", path).
			WithError(err).
			WithSuggestion("Verify file permissions and available disk space")
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
