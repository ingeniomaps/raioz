package config

import (
	"encoding/json"
	"os"
)

type Deps struct {
	SchemaVersion      string             `json:"schemaVersion"`
	Workspace          string             `json:"workspace,omitempty"`          // Optional workspace name. If not specified, uses Project.Name as workspace
	Network            NetworkConfig      `json:"network,omitempty"`            // Network configuration (shared by workspace). Optional - falls back to project.network for backward compatibility
	Project            Project            `json:"project"`
	Profiles           []string           `json:"profiles,omitempty"`           // Default profiles when running raioz up without --profile. If empty or missing, all services/infra are started
	Services           map[string]Service  `json:"services"`
	Infra              map[string]InfraEntry `json:"infra,omitempty"` // Each entry: string = path to YAML, or object = inline spec
	Env                EnvConfig           `json:"env"`
	ProjectComposePath string             `json:"projectComposePath,omitempty"` // Path to project's docker-compose.yml (if exists)
	ProjectRoot        string             `json:"projectRoot,omitempty"`        // Absolute path to project dir (where .raioz.json lives). Set at runtime when saving state; used when merging configs to resolve relative volumes per project

	// New fields for raioz.yaml (meta-orchestrator mode)
	Proxy       bool         `json:"proxy,omitempty"`       // Enable Caddy reverse proxy with HTTPS
	ProxyConfig *ProxyConfig `json:"-"`                     // Detailed proxy configuration (not serialized to JSON)
	PreHook     string       `json:"preHook,omitempty"`     // Command(s) to run before raioz up (e.g., fetch secrets)
	PostHook    string       `json:"postHook,omitempty"`    // Command(s) to run after raioz up (e.g., cleanup)
}

// LegacyProject represents the old structure where network was inside project
// Used for backward compatibility during unmarshaling
type LegacyProject struct {
	Name     string           `json:"name"`
	Network  NetworkConfig    `json:"network,omitempty"` // Legacy: network inside project
	Commands *ProjectCommands `json:"commands,omitempty"`
	Env      *EnvValue        `json:"env,omitempty"`
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
	Up     string               `json:"up,omitempty"`
	Down   string               `json:"down,omitempty"`
	Health string               `json:"health,omitempty"`
	Dev    *EnvironmentCommands `json:"dev,omitempty"`
	Prod   *EnvironmentCommands `json:"prod,omitempty"`
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
	Docker      *DockerConfig      `json:"docker,omitempty"`    // Optional: nil if source.command is set (host execution)
	DependsOn   []string           `json:"dependsOn,omitempty"` // At service level: for local/host services or to combine with docker.dependsOn
	Env         *EnvValue          `json:"env,omitempty"`       // Can be array of strings (file paths) or object (variables)
	Volumes     []string           `json:"volumes,omitempty"`   // For host services: symlinks in format "SRC:DEST" (SRC relative to projectDir, DEST relative to servicePath)
	Profiles    []string           `json:"profiles,omitempty"`
	Enabled     *bool              `json:"enabled,omitempty"`     // Explicit enable/disable (default: true)
	Mock        *MockConfig        `json:"mock,omitempty"`        // Mock configuration
	FeatureFlag *FeatureFlagConfig `json:"featureFlag,omitempty"` // Feature flag configuration
	Commands    *ServiceCommands   `json:"commands,omitempty"`    // Custom commands for launch/stop

	// New fields for raioz.yaml (meta-orchestrator mode)
	Watch          YAMLWatch      `json:"-"`                         // Watch config: true (raioz watches), "native" (service's own hot-reload)
	HealthEndpoint string         `json:"healthEndpoint,omitempty"`  // HTTP health check path (e.g., "/api/health")
	Hostname       string         `json:"hostname,omitempty"`        // Custom hostname for proxy (default: service name)
	Routing        *RoutingConfig `json:"routing,omitempty"`         // Proxy routing config (ws, sse, grpc, tunnel)
}

// GetDependsOn returns the effective dependsOn: service-level and docker-level merged (deduplicated).
// Use this for ordering, compose depends_on, and validation so both locations are honored.
func (s *Service) GetDependsOn() []string {
	seen := make(map[string]bool)
	var out []string
	for _, d := range s.DependsOn {
		if !seen[d] {
			seen[d] = true
			out = append(out, d)
		}
	}
	if s.Docker != nil {
		for _, d := range s.Docker.DependsOn {
			if !seen[d] {
				seen[d] = true
				out = append(out, d)
			}
		}
	}
	return out
}

type ServiceCommands struct {
	Up          string               `json:"up,omitempty"`
	Down        string               `json:"down,omitempty"`
	Health      string               `json:"health,omitempty"`
	Dev         *EnvironmentCommands `json:"dev,omitempty"`
	Prod        *EnvironmentCommands `json:"prod,omitempty"`
	ComposePath string               `json:"composePath,omitempty"` // Path to docker-compose.yml for services that use docker-compose commands
}

type SourceConfig struct {
	Kind    string `json:"kind"`                        // git | image | local
	Repo    string `json:"repo,omitempty"`              // Required if kind == "git"
	Branch  string `json:"branch,omitempty"`            // Required if kind == "git"
	Image   string `json:"image,omitempty"`             // Required if kind == "image"
	Tag     string `json:"tag,omitempty"`               // Required if kind == "image"
	Path    string `json:"path,omitempty"`              // Required if kind == "git" or "local"
	Access  string `json:"access,omitempty"`            // "readonly" | "editable" (default: "editable", only for git)
	Command string `json:"command,omitempty"`           // Command to run directly on host (without Docker)
	Runtime string `json:"runtime,omitempty"`           // Runtime type for host execution (optional)
}

type DockerConfig struct {
	Mode       string             `json:"mode,omitempty"` // "dev" | "prod" (optional if source.command is set)
	Ports      []string           `json:"ports,omitempty"`
	Volumes    []string           `json:"volumes,omitempty"`
	DependsOn  []string           `json:"dependsOn,omitempty"`
	Dockerfile string             `json:"dockerfile,omitempty"`
	Command    string             `json:"command,omitempty"`   // Command to run inside Docker container (for wrapper mode)
	Runtime    string             `json:"runtime,omitempty"`   // Runtime type for Docker wrapper mode (node, go, python, etc.)
	IP         string             `json:"ip,omitempty"`        // Static IP address in the network (e.g., "150.150.0.10")
	EnvVolume  string             `json:"envVolume,omitempty"` // Optional: mount .env file as volume at this path (e.g., "/app/.env")
	Healthcheck *HealthcheckConfig `json:"healthcheck,omitempty"` // Optional: same format as Docker Compose healthcheck
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

	// Unmarshal with legacy structure; each infra entry can be string (path to YAML) or object (inline)
	var legacyStruct struct {
		Project            LegacyProject        `json:"project"`
		Network            NetworkConfig        `json:"network,omitempty"`
		SchemaVersion      string               `json:"schemaVersion"`
		Workspace          string               `json:"workspace,omitempty"`
		Profiles           []string             `json:"profiles,omitempty"`
		Services           map[string]Service   `json:"services"`
		Infra              map[string]InfraEntry `json:"infra"`
		Env                EnvConfig            `json:"env"`
		ProjectComposePath string               `json:"projectComposePath,omitempty"`
	}
	if err := json.Unmarshal(data, &legacyStruct); err != nil {
		return nil, nil, err
	}
	if legacyStruct.Infra == nil {
		legacyStruct.Infra = make(map[string]InfraEntry)
	}

	// Build Deps struct
	deps := Deps{
		SchemaVersion: legacyStruct.SchemaVersion,
		Workspace:     legacyStruct.Workspace,
		Project: Project{
			Name:     legacyStruct.Project.Name,
			Commands: legacyStruct.Project.Commands,
			Env:      legacyStruct.Project.Env,
		},
		Profiles:           legacyStruct.Profiles,
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

// FilterByProfile filters services and infra by the given profile.
// Services/infra with no profiles are always included; otherwise only those matching the profile are included.
func FilterByProfile(deps *Deps, profile string) *Deps {
	filtered := &Deps{
		SchemaVersion:      deps.SchemaVersion,
		Workspace:          deps.Workspace,
		Network:            deps.Network,
		Project:            deps.Project,
		Services:           make(map[string]Service),
		Infra:              make(map[string]InfraEntry),
		Env:                deps.Env,
		ProjectComposePath: deps.ProjectComposePath,
	}

	for name, svc := range deps.Services {
		if svc.Enabled != nil && !*svc.Enabled {
			continue
		}
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

	for name, entry := range deps.Infra {
		profs := entry.Profiles()
		if len(profs) == 0 {
			filtered.Infra[name] = entry
		} else {
			for _, p := range profs {
				if p == profile {
					filtered.Infra[name] = entry
					break
				}
			}
		}
	}

	return filtered
}

// FilterByProfiles filters services and infra by a list of profiles (e.g. default profiles).
// Services/infra with no profiles are always included; otherwise included if at least one of their profiles is in the list.
func FilterByProfiles(deps *Deps, profiles []string) *Deps {
	if len(profiles) == 0 {
		return deps
	}
	allowed := make(map[string]bool)
	for _, p := range profiles {
		allowed[p] = true
	}
	filtered := &Deps{
		SchemaVersion:      deps.SchemaVersion,
		Workspace:          deps.Workspace,
		Network:            deps.Network,
		Project:            deps.Project,
		Profiles:           deps.Profiles,
		Services:           make(map[string]Service),
		Infra:              make(map[string]InfraEntry),
		Env:                deps.Env,
		ProjectComposePath: deps.ProjectComposePath,
	}
	for name, svc := range deps.Services {
		if svc.Enabled != nil && !*svc.Enabled {
			continue
		}
		if len(svc.Profiles) == 0 {
			filtered.Services[name] = svc
		} else {
			for _, p := range svc.Profiles {
				if allowed[p] {
					filtered.Services[name] = svc
					break
				}
			}
		}
	}
	for name, entry := range deps.Infra {
		profs := entry.Profiles()
		if len(profs) == 0 {
			filtered.Infra[name] = entry
		} else {
			for _, p := range profs {
				if allowed[p] {
					filtered.Infra[name] = entry
					break
				}
			}
		}
	}
	return filtered
}
