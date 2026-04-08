package cli

import (
	"path/filepath"
)

const defaultConfigName = ".raioz.json"

// ResolveConfigPath returns the path to use for the config file.
// If the given path is empty, returns default ".raioz.json".
// If a path is provided (any name or path), it is returned as-is (absolute when possible);
// no fallback to default — the loader will error if the file does not exist.
func ResolveConfigPath(path string) string {
	if path == "" {
		return defaultConfigName
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return abs
}
