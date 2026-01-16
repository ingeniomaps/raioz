package override

import (
	"encoding/json"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
)

const overridesFileName = "overrides.json"

// Override represents a service override configuration
type Override struct {
	Path   string `json:"path"`
	Mode   string `json:"mode"` // "local" | "image"
	Source string `json:"source"` // "external"
}

// Overrides represents the collection of service overrides
type Overrides map[string]Override

// getBaseDir returns the base directory for raioz (same logic as workspace.GetBaseDir)
func getBaseDir() (string, error) {
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

// GetOverridesPath returns the path to the overrides file
func GetOverridesPath() (string, error) {
	baseDir, err := getBaseDir()
	if err != nil {
		return "", fmt.Errorf("failed to get base directory for overrides: %w", err)
	}
	return filepath.Join(baseDir, overridesFileName), nil
}

// LoadOverrides loads overrides from ~/.raioz/overrides.json
func LoadOverrides() (Overrides, error) {
	path, err := GetOverridesPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(Overrides), nil
		}
		return nil, fmt.Errorf("failed to read overrides file: %w", err)
	}

	var overrides Overrides
	if err := json.Unmarshal(data, &overrides); err != nil {
		return nil, fmt.Errorf("failed to unmarshal overrides: %w", err)
	}

	// Initialize map if nil
	if overrides == nil {
		overrides = make(Overrides)
	}

	return overrides, nil
}

// SaveOverrides saves overrides to ~/.raioz/overrides.json
func SaveOverrides(overrides Overrides) error {
	path, err := GetOverridesPath()
	if err != nil {
		return err
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create overrides directory: %w", err)
	}

	data, err := json.MarshalIndent(overrides, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal overrides: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write overrides file: %w", err)
	}

	return nil
}

// AddOverride adds or updates an override for a service
func AddOverride(serviceName string, override Override) error {
	overrides, err := LoadOverrides()
	if err != nil {
		return err
	}

	overrides[serviceName] = override

	return SaveOverrides(overrides)
}

// RemoveOverride removes an override for a service
func RemoveOverride(serviceName string) error {
	overrides, err := LoadOverrides()
	if err != nil {
		return err
	}

	delete(overrides, serviceName)

	return SaveOverrides(overrides)
}

// GetOverride returns the override for a service if it exists
func GetOverride(serviceName string) (*Override, error) {
	overrides, err := LoadOverrides()
	if err != nil {
		return nil, err
	}

	if override, ok := overrides[serviceName]; ok {
		return &override, nil
	}

	return nil, nil
}

// HasOverride checks if a service has an override
func HasOverride(serviceName string) (bool, error) {
	overrides, err := LoadOverrides()
	if err != nil {
		return false, err
	}

	_, ok := overrides[serviceName]
	return ok, nil
}

// ValidateOverridePath validates that the override path exists
func ValidateOverridePath(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("override path does not exist: %s", path)
		}
		return fmt.Errorf("failed to stat override path: %w", err)
	}

	if !info.IsDir() {
		return fmt.Errorf("override path is not a directory: %s", path)
	}

	return nil
}

// CleanInvalidOverrides removes overrides where the path no longer exists
func CleanInvalidOverrides() ([]string, error) {
	overrides, err := LoadOverrides()
	if err != nil {
		return nil, err
	}

	var removed []string
	for serviceName, override := range overrides {
		if err := ValidateOverridePath(override.Path); err != nil {
			// Path doesn't exist, remove override
			delete(overrides, serviceName)
			removed = append(removed, serviceName)
		}
	}

	if len(removed) > 0 {
		if err := SaveOverrides(overrides); err != nil {
			return removed, fmt.Errorf("failed to save overrides after cleanup: %w", err)
		}
	}

	return removed, nil
}
