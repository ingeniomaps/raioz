package host

import (
	"os"
	"path/filepath"
	"strings"
)

// DetectComposePath detects the docker-compose.yml file path for a service
// It checks:
// 1. If composePath is explicitly specified in commands.composePath, use it (relative to servicePath)
// 2. If command contains docker-compose or docker compose, try to extract path from command
// 3. Search in servicePath for docker-compose.yml, docker-compose.yaml, compose.yml, compose.yaml
// 4. Search in common subdirectories: docker/, compose/, .docker/
func DetectComposePath(servicePath string, command string, explicitComposePath string) string {
	// 1. If explicitly specified, use it (resolve relative to servicePath)
	if explicitComposePath != "" {
		if filepath.IsAbs(explicitComposePath) {
			return explicitComposePath
		}
		// Relative path - resolve from servicePath
		resolved := filepath.Join(servicePath, explicitComposePath)
		if _, err := os.Stat(resolved); err == nil {
			return resolved
		}
	}

	// 2. Try to extract from command if it contains docker-compose
	if strings.Contains(command, "docker-compose") || strings.Contains(command, "docker compose") {
		// Try to find -f flag in command
		parts := strings.Fields(command)
		for i, part := range parts {
			if (part == "-f" || part == "--file") && i+1 < len(parts) {
				composeFile := parts[i+1]
				if filepath.IsAbs(composeFile) {
					if _, err := os.Stat(composeFile); err == nil {
						return composeFile
					}
				} else {
					// Relative path
					resolved := filepath.Join(servicePath, composeFile)
					if _, err := os.Stat(resolved); err == nil {
						return resolved
					}
				}
			}
		}
	}

	// 3. Search in servicePath for common compose file names
	composeFiles := []string{
		"docker-compose.yml",
		"docker-compose.yaml",
		"compose.yml",
		"compose.yaml",
	}
	for _, composeFile := range composeFiles {
		path := filepath.Join(servicePath, composeFile)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	// 4. Search in common subdirectories
	subdirs := []string{"docker", "compose", ".docker", "deploy", "deployment"}
	for _, subdir := range subdirs {
		subdirPath := filepath.Join(servicePath, subdir)
		if _, err := os.Stat(subdirPath); err == nil {
			for _, composeFile := range composeFiles {
				path := filepath.Join(subdirPath, composeFile)
				if _, err := os.Stat(path); err == nil {
					return path
				}
			}
		}
	}

	// Not found
	return ""
}
