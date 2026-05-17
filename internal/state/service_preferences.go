package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"raioz/internal/domain/models"
	raiozErrors "raioz/internal/errors"
	"raioz/internal/fsutil"
	"raioz/internal/workspace"
)

const servicePreferencesFileName = "service-preferences.json"

// ServicePreference / ServicePreferences live in internal/domain/models;
// aliases kept for callers (ADR-009).
type (
	ServicePreference  = models.ServicePreference
	ServicePreferences = models.ServicePreferences
)

// GetServicePreferencesPath returns the path to the service preferences file for a workspace.
// Preferences are stored per workspace (e.g. workspaces/roax/service-preferences.json).
func GetServicePreferencesPath(ws *workspace.Workspace) string {
	if ws == nil || ws.Root == "" {
		return ""
	}
	return filepath.Join(ws.Root, servicePreferencesFileName)
}

// LoadServicePreferences loads service preferences from the workspace directory
func LoadServicePreferences(ws *workspace.Workspace) (*ServicePreferences, error) {
	path := GetServicePreferencesPath(ws)
	if path == "" {
		return &ServicePreferences{Preferences: make(map[string]ServicePreference)}, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &ServicePreferences{
				Preferences: make(map[string]ServicePreference),
			}, nil
		}
		return nil, raiozErrors.New(raiozErrors.ErrCodeStateLoadError, "failed to read service preferences file").
			WithContext("path", path).
			WithError(err).
			WithSuggestion("Check that the service preferences file exists and is readable")
	}

	var prefs ServicePreferences
	if err := json.Unmarshal(data, &prefs); err != nil {
		return nil, raiozErrors.New(raiozErrors.ErrCodeStateLoadError, "failed to parse service preferences file").
			WithContext("path", path).
			WithError(err).
			WithSuggestion(
				"The service preferences file may be corrupted. " +
					"Try deleting it — preferences will be " +
					"re-created when needed",
			)
	}

	if prefs.Preferences == nil {
		prefs.Preferences = make(map[string]ServicePreference)
	}

	return &prefs, nil
}

// SaveServicePreferences saves service preferences to the workspace directory
func SaveServicePreferences(ws *workspace.Workspace, prefs *ServicePreferences) error {
	path := GetServicePreferencesPath(ws)
	if path == "" {
		return raiozErrors.New(raiozErrors.ErrCodeStateSaveError, "workspace root is required to save service preferences").
			WithSuggestion("Ensure you are running the command from within a valid workspace")
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return raiozErrors.New(raiozErrors.ErrCodeStateSaveError, "failed to create service preferences directory").
			WithContext("directory", dir).
			WithError(err).
			WithSuggestion("Verify file permissions and available disk space")
	}

	data, err := json.MarshalIndent(prefs, "", "  ")
	if err != nil {
		return raiozErrors.New(
			raiozErrors.ErrCodeStateSaveError,
			fmt.Sprintf("failed to marshal service preferences: %v", err),
		).WithContext("path", path).
			WithError(err).
			WithSuggestion(
				"The preferences data may be corrupted. " +
					"Try deleting the preferences file and " +
					"setting preferences again",
			)
	}

	if err := fsutil.WriteFileAtomic(path, data, 0600); err != nil {
		return raiozErrors.New(raiozErrors.ErrCodeStateSaveError, "failed to write service preferences file").
			WithContext("path", path).
			WithError(err).
			WithSuggestion("Verify file permissions and available disk space")
	}

	return nil
}

// GetServicePreference returns the preference for a specific service in the workspace
func GetServicePreference(ws *workspace.Workspace, serviceName string) (*ServicePreference, error) {
	prefs, err := LoadServicePreferences(ws)
	if err != nil {
		return nil, err
	}

	pref, exists := prefs.Preferences[serviceName]
	if !exists {
		return nil, nil
	}

	return &pref, nil
}

// SetServicePreference sets or updates a preference for a service in the workspace
func SetServicePreference(ws *workspace.Workspace, pref ServicePreference) error {
	prefs, err := LoadServicePreferences(ws)
	if err != nil {
		return err
	}

	if pref.Timestamp.IsZero() {
		pref.Timestamp = time.Now()
	}

	prefs.Preferences[pref.ServiceName] = pref

	return SaveServicePreferences(ws, prefs)
}

// RemoveServicePreference removes a preference for a service in the workspace
func RemoveServicePreference(ws *workspace.Workspace, serviceName string) error {
	prefs, err := LoadServicePreferences(ws)
	if err != nil {
		return err
	}

	delete(prefs.Preferences, serviceName)

	return SaveServicePreferences(ws, prefs)
}
