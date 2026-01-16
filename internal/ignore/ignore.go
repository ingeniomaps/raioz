package ignore

import (
	"encoding/json"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
)

const ignoreFileName = "ignore.json"

// IgnoreConfig represents the ignore configuration
type IgnoreConfig struct {
	Services []string `json:"services"` // List of ignored service names
}

// getBaseDirForIgnore returns the base directory for storing ignore file
// Uses same logic as workspace.GetBaseDir but specifically for config files
func getBaseDirForIgnore() (string, error) {
	// Check for override environment variable
	if home := os.Getenv("RAIOZ_HOME"); home != "" {
		if err := os.MkdirAll(home, 0755); err != nil {
			return "", fmt.Errorf("failed to create RAIOZ_HOME directory '%s': %w", home, err)
		}
		return home, nil
	}

	// Try /opt/raioz-proyecto first (preferred location)
	optBase := "/opt/raioz-proyecto"
	if err := os.MkdirAll(optBase, 0755); err == nil {
		return optBase, nil
	}

	// Failed to create in /opt, use fallback
	usr, err := user.Current()
	if err != nil {
		return "", fmt.Errorf("failed to get current user: %w", err)
	}

	homeDir := usr.HomeDir
	if homeDir == "" {
		return "", fmt.Errorf("home directory is empty")
	}

	fallbackBase := filepath.Join(homeDir, ".raioz")
	if runtime.GOOS == "windows" {
		fallbackBase = filepath.Join(homeDir, ".raioz")
	}

	if err := os.MkdirAll(fallbackBase, 0755); err != nil {
		return "", fmt.Errorf("failed to create fallback directory '%s': %w", fallbackBase, err)
	}

	return fallbackBase, nil
}

// GetIgnorePath returns the path to the ignore file
func GetIgnorePath() (string, error) {
	baseDir, err := getBaseDirForIgnore()
	if err != nil {
		return "", fmt.Errorf("failed to get base directory for ignore: %w", err)
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
