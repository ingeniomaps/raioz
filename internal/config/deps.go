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

// NetworkConfig represents a network configuration that can be either:
// - A string (network name): "mi-red"
// - An object with name and subnet: {"name": "mi-red", "subnet": "150.150.0.0/16"}
type NetworkConfig struct {
	Name   string // Network name (always present)
	Subnet string // Optional subnet (CIDR notation, e.g., "150.150.0.0/16")
	IsObject bool // True if this was deserialized as an object
}

// UnmarshalJSON implements custom JSON unmarshaling for NetworkConfig
func (n *NetworkConfig) UnmarshalJSON(data []byte) error {
	if n == nil {
		return fmt.Errorf("cannot unmarshal into nil NetworkConfig pointer")
	}

	// Try to unmarshal as string first (backward compatibility)
	var name string
	if err := json.Unmarshal(data, &name); err == nil {
		n.Name = name
		n.Subnet = ""
		n.IsObject = false
		return nil
	}

	// If not a string, try as object
	var obj struct {
		Name   string `json:"name"`
		Subnet string `json:"subnet"`
	}
	if err := json.Unmarshal(data, &obj); err == nil {
		if obj.Name == "" {
			return fmt.Errorf("network object must have a 'name' field")
		}
		n.Name = obj.Name
		n.Subnet = obj.Subnet
		n.IsObject = true
		return nil
	}

	return fmt.Errorf("network must be either a string (network name) or an object with 'name' and optional 'subnet' fields")
}

// MarshalJSON implements custom JSON marshaling for NetworkConfig
func (n NetworkConfig) MarshalJSON() ([]byte, error) {
	if !n.IsObject || n.Subnet == "" {
		return json.Marshal(n.Name)
	}
	return json.Marshal(map[string]string{
		"name":   n.Name,
		"subnet": n.Subnet,
	})
}

// GetName returns the network name
func (n *NetworkConfig) GetName() string {
	return n.Name
}

// GetSubnet returns the subnet if configured, or empty string
func (n *NetworkConfig) GetSubnet() string {
	return n.Subnet
}

// HasSubnet returns true if a subnet is configured
func (n *NetworkConfig) HasSubnet() bool {
	return n.Subnet != ""
}

type Deps struct {
	SchemaVersion      string             `json:"schemaVersion"`
	Workspace          string             `json:"workspace,omitempty"` // Optional workspace name. If not specified, uses Project.Name as workspace
	Network            NetworkConfig      `json:"network,omitempty"`  // Network configuration (shared by workspace). Optional - falls back to project.network for backward compatibility
	Project            Project            `json:"project"`
	Services           map[string]Service `json:"services"`
	Infra              map[string]Infra   `json:"infra"`
	Env                EnvConfig          `json:"env"`
	ProjectComposePath string             `json:"projectComposePath,omitempty"` // Path to project's docker-compose.yml (if exists)
}

// LegacyProject represents the old structure where network was inside project
// Used for backward compatibility during unmarshaling
type LegacyProject struct {
	Name     string            `json:"name"`
	Network  NetworkConfig     `json:"network,omitempty"` // Legacy: network inside project
	Commands *ProjectCommands  `json:"commands,omitempty"`
	Env      *EnvValue         `json:"env,omitempty"`
}

// GetWorkspaceName returns the workspace name for this project
// If Workspace is specified at root level, returns it. Otherwise, returns Project.Name.
func (d *Deps) GetWorkspaceName() string {
	if d.Workspace != "" {
		return d.Workspace
	}
	return d.Project.Name
}

// HasExplicitWorkspace returns true if workspace was explicitly set in config
func (d *Deps) HasExplicitWorkspace() bool {
	return d.Workspace != ""
}

type Project struct {
	Name     string           `json:"name"`
	Commands *ProjectCommands `json:"commands,omitempty"`
	Env      *EnvValue        `json:"env,omitempty"` // Project-level env files or variables
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
	Volumes     []string           `json:"volumes,omitempty"` // For host services: symlinks in format "SRC:DEST" (SRC relative to projectDir, DEST relative to servicePath)
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
	Kind    string `json:"kind"`              // git | image | local
	Repo    string `json:"repo"`              // Required if kind == "git"
	Branch  string `json:"branch"`            // Required if kind == "git"
	Image   string `json:"image"`             // Required if kind == "image"
	Tag     string `json:"tag"`               // Required if kind == "image"
	Path    string `json:"path"`              // Required if kind == "git" or "local"
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
	IP         string   `json:"ip,omitempty"`       // Static IP address in the network (e.g., "150.150.0.10")
	EnvVolume  string   `json:"envVolume,omitempty"` // Optional: mount .env file as volume at this path (e.g., "/app/.env")
}

type Infra struct {
	Image   string    `json:"image"`
	Tag     string    `json:"tag,omitempty"`
	Ports   []string  `json:"ports,omitempty"`   // Optional: can be null or empty array
	Volumes []string  `json:"volumes,omitempty"` // Optional: can be null or empty array
	Env     *EnvValue `json:"env,omitempty"`     // Can be array of strings (file paths) or object (variables)
	IP      string    `json:"ip,omitempty"`      // Static IP address in the network (e.g., "150.150.0.10")
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

	// Unmarshal with legacy structure to handle backward compatibility
	// This allows us to read network from either root level or project.network
	var legacyStruct struct {
		Project           LegacyProject       `json:"project"`
		Network           NetworkConfig       `json:"network,omitempty"`
		SchemaVersion     string              `json:"schemaVersion"`
		Workspace         string              `json:"workspace,omitempty"`
		Services          map[string]Service   `json:"services"`
		Infra             map[string]Infra     `json:"infra"`
		Env               EnvConfig           `json:"env"`
		ProjectComposePath string              `json:"projectComposePath,omitempty"`
	}
	if err := json.Unmarshal(data, &legacyStruct); err != nil {
		return nil, nil, err
	}

	// Build Deps struct
	deps := Deps{
		SchemaVersion:      legacyStruct.SchemaVersion,
		Workspace:          legacyStruct.Workspace,
		Project: Project{
			Name:     legacyStruct.Project.Name,
			Commands: legacyStruct.Project.Commands,
			Env:      legacyStruct.Project.Env,
		},
		Services:           legacyStruct.Services,
		Infra:              legacyStruct.Infra,
		Env:                legacyStruct.Env,
		ProjectComposePath: legacyStruct.ProjectComposePath,
	}

	// Migrate network: use root level if present, otherwise use project.network (backward compatibility)
	if legacyStruct.Network.Name != "" {
		deps.Network = legacyStruct.Network
	} else if legacyStruct.Project.Network.Name != "" {
		// Network is in project, migrate to root level
		deps.Network = legacyStruct.Project.Network
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
		SchemaVersion:      deps.SchemaVersion,
		Workspace:          deps.Workspace, // Preserve workspace
		Network:            deps.Network,   // Preserve network
		Project:            deps.Project,
		Services:           make(map[string]Service),
		Infra:              deps.Infra, // Infra is always included
		Env:                deps.Env,
		ProjectComposePath: deps.ProjectComposePath,
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
