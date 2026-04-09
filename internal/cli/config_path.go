package cli

import (
	"os"
	"path/filepath"
)

// AutoDetectMarker is returned by ResolveConfigPath when no config file is found,
// signaling that raioz should auto-detect the project structure.
const AutoDetectMarker = ":auto:"

// configCandidates lists config file names in priority order.
var configCandidates = []string{
	"raioz.yaml",
	"raioz.yml",
	".raioz.json",
}

// ResolveConfigPath returns the path to use for the config file.
// If the given path is empty, searches for raioz.yaml, raioz.yml, then .raioz.json.
// If none found, returns AutoDetectMarker to signal zero-config mode.
// If a path is provided, it is returned as-is (absolute when possible).
func ResolveConfigPath(path string) string {
	if path == "" {
		for _, candidate := range configCandidates {
			if _, err := os.Stat(candidate); err == nil {
				return candidate
			}
		}
		return AutoDetectMarker
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return abs
}

// IsAutoDetect returns true if the config path signals zero-config mode.
func IsAutoDetect(path string) bool {
	return path == AutoDetectMarker
}
