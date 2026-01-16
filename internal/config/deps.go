package config

import (
	"encoding/json"
	"fmt"
	"os"
)

// EnvValue represents an environment configuration that can be either:
// - An array of strings (file paths): ["local-deps", "services/shared"]
// - An object with variables: {"DATABASE_URL": "postgres://...", "API_KEY": "..."}
type EnvValue struct {
	Files     []string          // File paths (when env is an array of strings)
	Variables map[string]string // Direct variables (when env is an object)
	IsObject  bool              // True if this was deserialized as an object
}

// UnmarshalJSON implements custom JSON unmarshaling for EnvValue
func (e *EnvValue) UnmarshalJSON(data []byte) error {
	// Handle nil pointer case
	if e == nil {
		return fmt.Errorf("cannot unmarshal into nil EnvValue pointer")
	}

	// Try to unmarshal as array of strings first
	var files []string
	if err := json.Unmarshal(data, &files); err == nil {
		e.Files = files
		e.Variables = nil
		e.IsObject = false
		return nil
	}

	// If not an array, try as object
	var vars map[string]string
	if err := json.Unmarshal(data, &vars); err == nil {
		e.Files = nil
		e.Variables = vars
		e.IsObject = true
		return nil
	}

	// If neither works, return error
	return fmt.Errorf("env must be either an array of strings or an object with string values")
}

// MarshalJSON implements custom JSON marshaling for EnvValue
func (e EnvValue) MarshalJSON() ([]byte, error) {
	if e.IsObject && e.Variables != nil {
		return json.Marshal(e.Variables)
	}
	return json.Marshal(e.Files)
}

// GetFilePaths returns the file paths if this is a file-based config, or empty slice
func (e *EnvValue) GetFilePaths() []string {
	if e.IsObject {
		return nil
	}
	return e.Files
}

// GetVariables returns the variables if this is an object-based config, or nil
func (e *EnvValue) GetVariables() map[string]string {
	if !e.IsObject {
		return nil
	}
	return e.Variables
}

type Deps struct {
	SchemaVersion string             `json:"schemaVersion"`
	Project       Project            `json:"project"`
	Services      map[string]Service `json:"services"`
	Infra         map[string]Infra   `json:"infra"`
	Env           EnvConfig          `json:"env"`
}

type Project struct {
	Name     string            `json:"name"`
	Network  string            `json:"network"`
	Commands *ProjectCommands   `json:"commands,omitempty"`
}

type ProjectCommands struct {
	Up     string                 `json:"up,omitempty"`
	Down   string                 `json:"down,omitempty"`
	Health string                 `json:"health,omitempty"`
	Dev    *EnvironmentCommands   `json:"dev,omitempty"`
	Prod   *EnvironmentCommands   `json:"prod,omitempty"`
}

type EnvironmentCommands struct {
	Up     string `json:"up,omitempty"`
	Down   string `json:"down,omitempty"`
	Health string `json:"health,omitempty"`
}

type EnvConfig struct {
	UseGlobal bool              `json:"useGlobal"`
	Files     []string          `json:"files"`
	Variables map[string]string `json:"variables,omitempty"` // Direct variables to write to global.env
}

type Service struct {
	Source      SourceConfig       `json:"source"`
	Docker      *DockerConfig      `json:"docker,omitempty"` // Optional: nil if source.command is set (host execution)
	Env         *EnvValue          `json:"env,omitempty"`     // Can be array of strings (file paths) or object (variables)
	Profiles    []string           `json:"profiles,omitempty"`
	Enabled     *bool              `json:"enabled,omitempty"`     // Explicit enable/disable (default: true)
	Mock        *MockConfig        `json:"mock,omitempty"`        // Mock configuration
	FeatureFlag *FeatureFlagConfig `json:"featureFlag,omitempty"` // Feature flag configuration
	Commands    *ServiceCommands   `json:"commands,omitempty"`     // Custom commands for launch/stop
}

type ServiceCommands struct {
	Up         string                 `json:"up,omitempty"`
	Down       string                 `json:"down,omitempty"`
	Health     string                 `json:"health,omitempty"`
	Dev        *EnvironmentCommands   `json:"dev,omitempty"`
	Prod       *EnvironmentCommands   `json:"prod,omitempty"`
	ComposePath string                `json:"composePath,omitempty"` // Path to docker-compose.yml for services that use docker-compose commands
}

type SourceConfig struct {
	Kind    string `json:"kind"`              // git | image
	Repo    string `json:"repo"`              // Required if kind == "git"
	Branch  string `json:"branch"`            // Required if kind == "git"
	Image   string `json:"image"`             // Required if kind == "image"
	Tag     string `json:"tag"`               // Required if kind == "image"
	Path    string `json:"path"`              // Required if kind == "git"
	Access  string `json:"access,omitempty"`  // "readonly" | "editable" (default: "editable", only for git)
	Command string `json:"command,omitempty"` // Command to run directly on host (without Docker)
	Runtime string `json:"runtime,omitempty"` // Runtime type for host execution (optional)
}

type DockerConfig struct {
	Mode       string   `json:"mode,omitempty"`       // "dev" | "prod" (optional if source.command is set)
	Ports      []string `json:"ports,omitempty"`
	Volumes    []string `json:"volumes,omitempty"`
	DependsOn  []string `json:"dependsOn,omitempty"`
	Dockerfile string   `json:"dockerfile,omitempty"`
	Command    string   `json:"command,omitempty"` // Command to run inside Docker container (for wrapper mode)
	Runtime    string   `json:"runtime,omitempty"` // Runtime type for Docker wrapper mode (node, go, python, etc.)
}

type Infra struct {
	Image   string    `json:"image"`
	Tag     string    `json:"tag"`
	Ports   []string  `json:"ports"`
	Volumes []string  `json:"volumes"`
	Env     *EnvValue `json:"env,omitempty"` // Can be array of strings (file paths) or object (variables)
}

func LoadDeps(path string) (*Deps, []string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}

	// Check for deprecated fields and collect warnings
	warnings, err := CheckDeprecatedFields(data)
	if err != nil {
		// Non-fatal, continue loading
		warnings = []string{}
	}

	var deps Deps
	if err := json.Unmarshal(data, &deps); err != nil {
		return nil, nil, err
	}

	return &deps, warnings, nil
}

// LoadDepsLegacy is a compatibility wrapper that ignores warnings
// Deprecated: Use LoadDeps instead to get deprecation warnings
func LoadDepsLegacy(path string) (*Deps, error) {
	deps, _, err := LoadDeps(path)
	return deps, err
}

// FilterByProfile filters services by the given profile
func FilterByProfile(deps *Deps, profile string) *Deps {
	filtered := &Deps{
		SchemaVersion: deps.SchemaVersion,
		Project:       deps.Project,
		Services:      make(map[string]Service),
		Infra:         deps.Infra, // Infra is always included
		Env:           deps.Env,
	}

	for name, svc := range deps.Services {
		// Skip disabled services (enabled: false takes precedence)
		if svc.Enabled != nil && !*svc.Enabled {
			continue
		}

		// Include service if it has no profiles (always included) or if it matches the profile
		if len(svc.Profiles) == 0 {
			filtered.Services[name] = svc
		} else {
			for _, p := range svc.Profiles {
				if p == profile {
					filtered.Services[name] = svc
					break
				}
			}
		}
	}

	return filtered
}
