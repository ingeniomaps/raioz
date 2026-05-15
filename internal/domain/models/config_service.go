package models

import "os"

// Service describes a unit of local code that raioz orchestrates.
type Service struct {
	Source      SourceConfig       `json:"source"`
	Docker      *DockerConfig      `json:"docker,omitempty"`    // nil if host execution
	DependsOn   []string           `json:"dependsOn,omitempty"` // Service-level deps
	Env         *EnvValue          `json:"env,omitempty"`       // File paths or variables
	Volumes     []string           `json:"volumes,omitempty"`   // Symlinks "SRC:DEST"
	Profiles    []string           `json:"profiles,omitempty"`
	Enabled     *bool              `json:"enabled,omitempty"`     // Default: true
	Mock        *MockConfig        `json:"mock,omitempty"`        // Mock config
	FeatureFlag *FeatureFlagConfig `json:"featureFlag,omitempty"` // Feature flag config
	Commands    *ServiceCommands   `json:"commands,omitempty"`    // Custom commands

	// New fields for raioz.yaml (meta-orchestrator mode)
	Watch          YAMLWatch `json:"-"`                        // Watch config
	HealthEndpoint string    `json:"healthEndpoint,omitempty"` // e.g. "/api/health"
	Hostname       string    `json:"hostname,omitempty"`       // Custom proxy hostname
	// HostnameAliases exposes the same upstream under extra subdomains.
	// Populated from `hostnameAliases:` in raioz.yaml. Empty means the
	// service is only reachable through Hostname.
	HostnameAliases []string       `json:"hostnameAliases,omitempty"`
	Routing         *RoutingConfig `json:"routing,omitempty"` // Proxy routing config

	// ProxyOverride forces a specific (target, port) pair for the proxy
	// reverse_proxy directive, bypassing runtime detection. Needed when a
	// service's `command:` launches its own docker compose that raioz
	// can't introspect — see BUG-13.
	ProxyOverride *ServiceProxyOverride `json:"proxyOverride,omitempty"`

	// Port is the explicit host port the user declared in raioz.yaml (`port:`).
	// 0 means "unset — let raioz infer and allocate". See the allocator in
	// internal/app/upcase/port_alloc.go for precedence rules.
	Port int `json:"port,omitempty"`
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

// ServiceCommands holds per-service launch verbs.
type ServiceCommands struct {
	Up          string               `json:"up,omitempty"`
	Down        string               `json:"down,omitempty"`
	Health      string               `json:"health,omitempty"`
	Dev         *EnvironmentCommands `json:"dev,omitempty"`
	Prod        *EnvironmentCommands `json:"prod,omitempty"`
	ComposePath string               `json:"composePath,omitempty"` // docker-compose.yml path
}

// SourceConfig describes where the code for a Service lives.
type SourceConfig struct {
	Kind         string   `json:"kind"`                   // git | image | local
	Repo         string   `json:"repo,omitempty"`         // Required if kind == "git"
	Branch       string   `json:"branch,omitempty"`       // Required if kind == "git"
	Image        string   `json:"image,omitempty"`        // Required if kind == "image"
	Tag          string   `json:"tag,omitempty"`          // Required if kind == "image"
	Path         string   `json:"path,omitempty"`         // Required if kind == "git" or "local"
	Access       string   `json:"access,omitempty"`       // "readonly" | "editable" (default: "editable", only for git)
	Auth         string   `json:"auth,omitempty"`         // "" (strict, default) | "inherit" | "gh" | "ssh" — issue 067
	Command      string   `json:"command,omitempty"`      // Command to run directly on host (without Docker)
	Runtime      string   `json:"runtime,omitempty"`      // Runtime type for host execution (optional)
	ComposeFiles []string `json:"composeFiles,omitempty"` // Explicit compose files (overrides auto-detect)
}

// DockerConfig captures Docker-specific settings for a Service.
type DockerConfig struct {
	Mode        string             `json:"mode,omitempty"` // "dev" | "prod" (optional if source.command is set)
	Ports       []string           `json:"ports,omitempty"`
	Volumes     []string           `json:"volumes,omitempty"`
	DependsOn   []string           `json:"dependsOn,omitempty"`
	Dockerfile  string             `json:"dockerfile,omitempty"`
	Command     string             `json:"command,omitempty"`     // Command inside container
	Runtime     string             `json:"runtime,omitempty"`     // node, go, python, etc.
	IP          string             `json:"ip,omitempty"`          // Static IP (e.g. "150.150.0.10")
	EnvVolume   string             `json:"envVolume,omitempty"`   // Mount .env at this path
	Healthcheck *HealthcheckConfig `json:"healthcheck,omitempty"` // Optional: same format as Docker Compose healthcheck
}

// YAMLWatch can be a bool (true/false) or a string ("native").
type YAMLWatch struct {
	Enabled bool
	Mode    string // "" (raioz watches), "native" (service has its own hot-reload)
}

// UnmarshalYAML implements yaml.Unmarshaler for YAMLWatch.
func (w *YAMLWatch) UnmarshalYAML(unmarshal func(any) error) error {
	var b bool
	if err := unmarshal(&b); err == nil {
		w.Enabled = b
		w.Mode = ""
		return nil
	}

	var s string
	if err := unmarshal(&s); err == nil {
		w.Enabled = true
		w.Mode = s
		return nil
	}

	return nil
}

// MockConfig defines configuration for a mock service.
type MockConfig struct {
	Enabled bool     `json:"enabled,omitempty"` // Whether to use mock
	Image   string   `json:"image,omitempty"`   // Docker image for mock
	Tag     string   `json:"tag,omitempty"`     // Tag for mock image
	Ports   []string `json:"ports,omitempty"`   // Ports for mock (overrides service ports)
	Env     []string `json:"env,omitempty"`     // Env vars for mock
}

// FeatureFlagConfig defines feature flag configuration.
type FeatureFlagConfig struct {
	Enabled  bool     `json:"enabled,omitempty"`  // Direct enable/disable
	EnvVar   string   `json:"envVar,omitempty"`   // Environment variable to check
	EnvValue string   `json:"envValue,omitempty"` // Required value for envVar
	Profiles []string `json:"profiles,omitempty"` // Enabled for specific profiles
	Disabled bool     `json:"disabled,omitempty"` // Direct disable (takes precedence)
}

// IsEnabled checks if a feature flag is enabled based on configuration and context.
func (f *FeatureFlagConfig) IsEnabled(profile string, envVars map[string]string) bool {
	if f.Disabled {
		return false
	}

	if f.Enabled {
		if len(f.Profiles) > 0 {
			for _, p := range f.Profiles {
				if p == profile {
					return true
				}
			}
			return false
		}
		return true
	}

	if f.EnvVar != "" {
		envValue, exists := envVars[f.EnvVar]
		if !exists {
			envValue = os.Getenv(f.EnvVar)
		}
		if f.EnvValue != "" {
			return envValue == f.EnvValue
		}
		return envValue != "" && envValue != "false" && envValue != "0"
	}

	if len(f.Profiles) > 0 {
		for _, p := range f.Profiles {
			if p == profile {
				return true
			}
		}
	}

	return true
}
