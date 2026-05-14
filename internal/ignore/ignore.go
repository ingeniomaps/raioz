package ignore

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"raioz/internal/naming"
)

const ignoreFileName = "ignore.json"

// IgnoreConfig represents the ignore configuration
type IgnoreConfig struct {
	Services []string `json:"services"` // List of ignored service names
}

// GetIgnorePath returns the path to the ignore file.
//
// Delegates location selection to naming.RaiozStateDir() (ADR-022)
// so audit/ignore/workspace agree on the same root.
func GetIgnorePath() (string, error) {
	baseDir := naming.RaiozStateDir()
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create ignore state dir %q: %w", baseDir, err)
	}
	return filepath.Join(baseDir, ignoreFileName), nil
}

// Load loads the ignore configuration
func Load() (*IgnoreConfig, error) {
	path, err := GetIgnorePath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist, return empty config
			return &IgnoreConfig{Services: []string{}}, nil
		}
		return nil, fmt.Errorf("failed to read ignore file: %w", err)
	}

	var config IgnoreConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal ignore file: %w", err)
	}

	// Initialize services slice if nil
	if config.Services == nil {
		config.Services = []string{}
	}

	return &config, nil
}

// Save saves the ignore configuration
func Save(config *IgnoreConfig) error {
	path, err := GetIgnorePath()
	if err != nil {
		return err
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory for ignore file: %w", err)
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal ignore file: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write ignore file: %w", err)
	}

	return nil
}

// IsIgnored checks if a service is ignored
func IsIgnored(serviceName string) (bool, error) {
	config, err := Load()
	if err != nil {
		return false, err
	}

	for _, ignored := range config.Services {
		if ignored == serviceName {
			return true, nil
		}
	}

	return false, nil
}

// AddService adds a service to the ignore list
func AddService(serviceName string) error {
	config, err := Load()
	if err != nil {
		return err
	}

	// Check if already ignored
	for _, ignored := range config.Services {
		if ignored == serviceName {
			return nil // Already ignored, no-op
		}
	}

	// Add to list
	config.Services = append(config.Services, serviceName)

	return Save(config)
}

// RemoveService removes a service from the ignore list
func RemoveService(serviceName string) error {
	config, err := Load()
	if err != nil {
		return err
	}

	// Find and remove
	var newServices []string
	found := false
	for _, ignored := range config.Services {
		if ignored != serviceName {
			newServices = append(newServices, ignored)
		} else {
			found = true
		}
	}

	if !found {
		return nil // Not in list, no-op
	}

	config.Services = newServices
	return Save(config)
}

// GetIgnoredServices returns the list of ignored services
func GetIgnoredServices() ([]string, error) {
	config, err := Load()
	if err != nil {
		return nil, err
	}

	return config.Services, nil
}
