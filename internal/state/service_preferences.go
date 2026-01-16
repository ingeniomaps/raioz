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
	ServiceName string    `json:"serviceName"` // Name of the service (e.g., "nginx")
	Preference  string    `json:"preference"`  // "local" | "cloned" | "ask"
	ProjectPath string    `json:"projectPath,omitempty"` // Path to local project (if preference is "local")
	Workspace   string    `json:"workspace,omitempty"`   // Workspace name (if preference is "cloned")
	Reason      string    `json:"reason,omitempty"`      // Reason for the preference
	Timestamp   time.Time `json:"timestamp"`
}

// ServicePreferences represents all service preferences
type ServicePreferences struct {
	Preferences map[string]ServicePreference `json:"preferences"` // Key: serviceName
}

// GetServicePreferencesPath returns the path to the service preferences file
func GetServicePreferencesPath() (string, error) {
	base, err := workspace.GetBaseDir()
	if err != nil {
		return "", fmt.Errorf("failed to get base directory: %w", err)
	}
	return filepath.Join(base, servicePreferencesFileName), nil
}

// LoadServicePreferences loads service preferences from disk
func LoadServicePreferences() (*ServicePreferences, error) {
	path, err := GetServicePreferencesPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Return empty preferences if file doesn't exist
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

	// Ensure map is initialized
	if prefs.Preferences == nil {
		prefs.Preferences = make(map[string]ServicePreference)
	}

	return &prefs, nil
}

// SaveServicePreferences saves service preferences to disk
func SaveServicePreferences(prefs *ServicePreferences) error {
	path, err := GetServicePreferencesPath()
	if err != nil {
		return err
	}

	// Ensure directory exists (use 0700 for security - owner only)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create preferences directory: %w", err)
	}

	data, err := json.MarshalIndent(prefs, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal service preferences: %w", err)
	}

	// Use 0600 permissions (read/write for owner only) for security
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write service preferences: %w", err)
	}

	return nil
}

// GetServicePreference returns the preference for a specific service
func GetServicePreference(serviceName string) (*ServicePreference, error) {
	prefs, err := LoadServicePreferences()
	if err != nil {
		return nil, err
	}

	pref, exists := prefs.Preferences[serviceName]
	if !exists {
		return nil, nil // No preference set
	}

	return &pref, nil
}

// SetServicePreference sets or updates a preference for a service
func SetServicePreference(pref ServicePreference) error {
	prefs, err := LoadServicePreferences()
	if err != nil {
		return err
	}

	// Set timestamp if not set
	if pref.Timestamp.IsZero() {
		pref.Timestamp = time.Now()
	}

	prefs.Preferences[pref.ServiceName] = pref

	return SaveServicePreferences(prefs)
}

// RemoveServicePreference removes a preference for a service
func RemoveServicePreference(serviceName string) error {
	prefs, err := LoadServicePreferences()
	if err != nil {
		return err
	}

	delete(prefs.Preferences, serviceName)

	return SaveServicePreferences(prefs)
}
