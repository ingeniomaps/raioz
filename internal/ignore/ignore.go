package ignore

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"raioz/internal/fsutil"
	"raioz/internal/naming"
)

const ignoreFileName = "ignore.json"

// IgnoreConfig represents the ignore configuration
type IgnoreConfig struct {
	// Services is the list from before ignores were per project. It still
	// applies to every project, so nothing ignored then comes back on its
	// own; new entries go under Projects.
	Services []string `json:"services"`
	// Projects maps a project to the services ignored in it. Ignoring
	// `api` in one project says nothing about another project's `api`.
	Projects map[string][]string `json:"projects,omitempty"`
}

// GetIgnorePath returns the path to the ignore file.
// Location delegated to naming.RaiozStateDir() — ADR-022.
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

	if err := fsutil.WriteFileAtomic(path, data, 0644); err != nil {
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

// ForProject returns the services ignored in a project: its own list plus
// the legacy global one. An empty project means the legacy list alone.
func ForProject(project string) ([]string, error) {
	config, err := Load()
	if err != nil {
		return nil, err
	}
	out := append([]string{}, config.Services...)
	for _, name := range config.Projects[project] {
		if !contains(out, name) {
			out = append(out, name)
		}
	}
	return out, nil
}

// AddFor ignores a service in one project. An empty project falls back to
// the legacy global list.
func AddFor(project, serviceName string) error {
	if project == "" {
		return AddService(serviceName)
	}
	config, err := Load()
	if err != nil {
		return err
	}
	if contains(config.Projects[project], serviceName) {
		return nil
	}
	if config.Projects == nil {
		config.Projects = map[string][]string{}
	}
	config.Projects[project] = append(config.Projects[project], serviceName)
	return Save(config)
}

// RemoveFor stops ignoring a service in one project. The legacy global
// entry goes too: it applied to this project, and leaving it would make
// the removal a no-op.
func RemoveFor(project, serviceName string) error {
	config, err := Load()
	if err != nil {
		return err
	}
	config.Services = without(config.Services, serviceName)
	if project != "" && config.Projects != nil {
		config.Projects[project] = without(config.Projects[project], serviceName)
		if len(config.Projects[project]) == 0 {
			delete(config.Projects, project)
		}
	}
	return Save(config)
}

func contains(list []string, name string) bool {
	for _, item := range list {
		if item == name {
			return true
		}
	}
	return false
}

func without(list []string, name string) []string {
	out := make([]string, 0, len(list))
	for _, item := range list {
		if item != name {
			out = append(out, item)
		}
	}
	return out
}
