package docker

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// GetVolumeProjects returns projects that use a specific volume name.
func GetVolumeProjects(volumeName string, baseDir string) ([]string, error) {
	var projects []string
	workspacesDir := filepath.Join(baseDir, "workspaces")

	// Read workspaces directory
	entries, err := os.ReadDir(workspacesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return projects, nil // No workspaces yet
		}
		return nil, fmt.Errorf("failed to read workspaces: %w", err)
	}

	// Check each workspace for state file
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		projectName := entry.Name()
		statePath := filepath.Join(workspacesDir, projectName, ".state.json")

		// Try to load state
		data, err := os.ReadFile(statePath)
		if err != nil {
			continue // Skip if can't read
		}

		// Parse JSON to check volumes
		var state struct {
			Services map[string]struct {
				Docker struct {
					Volumes []string `json:"volumes"`
				} `json:"docker"`
			} `json:"services"`
			Infra map[string]struct {
				Volumes []string `json:"volumes"`
			} `json:"infra"`
		}

		if err := json.Unmarshal(data, &state); err != nil {
			continue // Skip if invalid JSON
		}

		// Check services
		for _, svc := range state.Services {
			namedVols, _ := ExtractNamedVolumes(svc.Docker.Volumes)
			for _, vol := range namedVols {
				if vol == volumeName {
					projects = append(projects, projectName)
					goto nextProject
				}
			}
		}

		// Check infra
		for _, infra := range state.Infra {
			namedVols, _ := ExtractNamedVolumes(infra.Volumes)
			for _, vol := range namedVols {
				if vol == volumeName {
					projects = append(projects, projectName)
					goto nextProject
				}
			}
		}

	nextProject:
	}

	return projects, nil
}
