package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"raioz/internal/config"
	"raioz/internal/workspace"
)

const globalStateFileName = "state.json"

// GlobalState represents the global state across all projects
type GlobalState struct {
	ActiveProjects []string              `json:"activeProjects"`
	Projects       map[string]ProjectState `json:"projects"`
}

// ProjectState represents the state of a single project
type ProjectState struct {
	Name          string            `json:"name"`
	Workspace     string            `json:"workspace"`
	LastExecution time.Time         `json:"lastExecution"`
	Services      []ServiceState    `json:"services"`
}

// ServiceState represents the state of a single service
type ServiceState struct {
	Name    string `json:"name"`
	Mode    string `json:"mode"` // dev or prod
	Version string `json:"version"` // Commit SHA or image tag
	Image   string `json:"image,omitempty"` // Full image name (if applicable)
	Status  string `json:"status"` // running or stopped
}

// GetGlobalStatePath returns the path to the global state file
func GetGlobalStatePath() (string, error) {
	base, err := workspace.GetBaseDir()
	if err != nil {
		return "", fmt.Errorf("failed to get base directory: %w", err)
	}
	return filepath.Join(base, globalStateFileName), nil
}

// LoadGlobalState loads the global state from disk
func LoadGlobalState() (*GlobalState, error) {
	path, err := GetGlobalStatePath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Return empty state if file doesn't exist
			return &GlobalState{
				ActiveProjects: []string{},
				Projects:       make(map[string]ProjectState),
			}, nil
		}
		return nil, fmt.Errorf("failed to read global state: %w", err)
	}

	var state GlobalState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to unmarshal global state: %w", err)
	}

	// Ensure maps are initialized
	if state.Projects == nil {
		state.Projects = make(map[string]ProjectState)
	}
	if state.ActiveProjects == nil {
		state.ActiveProjects = []string{}
	}

	return &state, nil
}

// SaveGlobalState saves the global state to disk
func SaveGlobalState(state *GlobalState) error {
	path, err := GetGlobalStatePath()
	if err != nil {
		return err
	}

	// Ensure directory exists (use 0700 for security - owner only)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create state directory: %w", err)
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal global state: %w", err)
	}

	// Use 0600 permissions (read/write for owner only) for security
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write global state: %w", err)
	}

	return nil
}

// GetActiveProjects returns a list of active project names
func GetActiveProjects() ([]string, error) {
	state, err := LoadGlobalState()
	if err != nil {
		return nil, err
	}
	return state.ActiveProjects, nil
}

// GetProjectState returns the state of a specific project
func GetProjectState(projectName string) (*ProjectState, error) {
	state, err := LoadGlobalState()
	if err != nil {
		return nil, err
	}

	projectState, exists := state.Projects[projectName]
	if !exists {
		return nil, fmt.Errorf("project %s not found in global state", projectName)
	}

	return &projectState, nil
}

// UpdateProjectState updates the state of a project in the global state
func UpdateProjectState(projectName string, projectState ProjectState) error {
	state, err := LoadGlobalState()
	if err != nil {
		return err
	}

	// Update project state
	state.Projects[projectName] = projectState

	// Update active projects list (add if not present)
	found := false
	for _, name := range state.ActiveProjects {
		if name == projectName {
			found = true
			break
		}
	}
	if !found {
		state.ActiveProjects = append(state.ActiveProjects, projectName)
	}

	return SaveGlobalState(state)
}

// RemoveProject removes a project from the global state
func RemoveProject(projectName string) error {
	state, err := LoadGlobalState()
	if err != nil {
		return err
	}

	// Remove from projects map
	delete(state.Projects, projectName)

	// Remove from active projects list
	var newActiveProjects []string
	for _, name := range state.ActiveProjects {
		if name != projectName {
			newActiveProjects = append(newActiveProjects, name)
		}
	}
	state.ActiveProjects = newActiveProjects

	return SaveGlobalState(state)
}

// UpdateLastExecution updates the last execution timestamp for a project
func UpdateLastExecution(projectName string) error {
	state, err := LoadGlobalState()
	if err != nil {
		return err
	}

	projectState, exists := state.Projects[projectName]
	if !exists {
		// Create new project state if it doesn't exist
		projectState = ProjectState{
			Name:          projectName,
			LastExecution: time.Now(),
			Services:      []ServiceState{},
		}
	} else {
		projectState.LastExecution = time.Now()
	}

	return UpdateProjectState(projectName, projectState)
}

// BuildServiceStates builds ServiceState list from deps and service info
// serviceInfos can be nil - in that case, only basic info from deps is used
func BuildServiceStates(
	deps *config.Deps,
	serviceInfos map[string]*ServiceInfo,
) []ServiceState {
	var serviceStates []ServiceState

	for name, svc := range deps.Services {
		serviceState := ServiceState{
			Name:   name,
			Mode:   "dev", // Default mode
			Status: "stopped",
		}

		// Set mode from docker config if available
		if svc.Docker != nil && svc.Docker.Mode != "" {
			serviceState.Mode = svc.Docker.Mode
		}

		// Get info from serviceInfos if available
		if serviceInfos != nil {
			if info, ok := serviceInfos[name]; ok {
				serviceState.Status = info.Status
				serviceState.Version = info.Version
				serviceState.Image = info.Image
			}
		}

		// Fallback: set image info from deps if not set from serviceInfos
		if serviceState.Image == "" && svc.Source.Kind == "image" {
			serviceState.Image = svc.Source.Image
			if svc.Source.Tag != "" {
				serviceState.Image = svc.Source.Image + ":" + svc.Source.Tag
				if serviceState.Version == "" {
					serviceState.Version = svc.Source.Tag
				}
			}
		}

		serviceStates = append(serviceStates, serviceState)
	}

	return serviceStates
}

// ServiceInfo is a minimal interface for service information
// This avoids circular dependency with docker package
type ServiceInfo struct {
	Status  string
	Version string
	Image   string
}
