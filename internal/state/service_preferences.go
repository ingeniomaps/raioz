package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"raioz/internal/workspace"
)

const servicePreferencesFileName = "service-preferences.json"

// ServicePreference represents a user's preference for handling service conflicts
type ServicePreference struct {
	ServiceName string    `json:"serviceName"`           // Name of the service (e.g., "nginx")
	Preference  string    `json:"preference"`            // "local" | "cloned" | "ask"
	ProjectPath string    `json:"projectPath,omitempty"` // Path to local project (if preference is "local")
	Workspace   string    `json:"workspace,omitempty"`   // Workspace name (if preference is "cloned")
	Reason      string    `json:"reason,omitempty"`      // Reason for the preference
	Timestamp   time.Time `json:"timestamp"`
}

// ServicePreferences represents all service preferences
type ServicePreferences struct {
	Preferences map[string]ServicePreference `json:"preferences"` // Key: serviceName
}

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
		return nil, fmt.Errorf("failed to read service preferences: %w", err)
	}

	var prefs ServicePreferences
	if err := json.Unmarshal(data, &prefs); err != nil {
		return nil, fmt.Errorf("failed to unmarshal service preferences: %w", err)
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
		return fmt.Errorf("workspace root is required to save service preferences")
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create preferences directory: %w", err)
	}

	data, err := json.MarshalIndent(prefs, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal service preferences: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write service preferences: %w", err)
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
