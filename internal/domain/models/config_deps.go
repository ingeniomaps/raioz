package models

// SourceFormat marks the on-disk shape the loader produced. Orthogonal
// to the yaml `version:` field. See ADR-039.
type SourceFormat string

const (
	SourceFormatLegacyJSON SourceFormat = "legacy-json"
	SourceFormatYAML       SourceFormat = "yaml"
)

// Deps is the canonical raioz project description. Built either from a
// .raioz.json file (legacy) or from a raioz.yaml via the YAML bridge.
type Deps struct {
	// SchemaVersion is the legacy "1.0"/"2.0" discriminator. New code
	// MUST read SourceFormat instead; SchemaVersion goes away in v1.0
	// alongside the JSON loader (ADR-038, ADR-039).
	SchemaVersion      string                `json:"schemaVersion"`
	SourceFormat       SourceFormat          `json:"-"` // canonical discriminator (ADR-039)
	Workspace          string                `json:"workspace,omitempty"` // Optional workspace name
	Network            NetworkConfig         `json:"network,omitempty"`   // Network config (shared by workspace)
	Project            Project               `json:"project"`
	Profiles           []string              `json:"profiles,omitempty"` // Default profiles for raioz up
	Services           map[string]Service    `json:"services"`
	Infra              map[string]InfraEntry `json:"infra,omitempty"` // string=YAML path or object=inline
	Env                EnvConfig             `json:"env"`
	ProjectComposePath string                `json:"projectComposePath,omitempty"` // docker-compose.yml path
	ProjectRoot        string                `json:"projectRoot,omitempty"`        // Absolute path to project dir

	// New fields for raioz.yaml (meta-orchestrator mode)
	Proxy       bool         `json:"proxy,omitempty"`     // Enable Caddy reverse proxy
	ProxyConfig *ProxyConfig `json:"-"`                   // Detailed proxy config (not serialized)
	PreHook     string       `json:"preHook,omitempty"`   // Run before raioz up
	PreUpHook   string       `json:"preUpHook,omitempty"` // Run after infra/sibling-spawn, before services (ADR-024)
	PostHook    string       `json:"postHook,omitempty"`  // Run after raioz up
}

// GetWorkspaceName returns the workspace name for this project.
// If Workspace is specified at root level, returns it. Otherwise, returns Project.Name.
func (d *Deps) GetWorkspaceName() string {
	if d.Workspace != "" {
		return d.Workspace
	}
	return d.Project.Name
}

// HasExplicitWorkspace returns true if workspace was explicitly set in config.
func (d *Deps) HasExplicitWorkspace() bool {
	return d.Workspace != ""
}

// Project carries the project identity and its optional command overrides.
type Project struct {
	Name     string           `json:"name"`
	Commands *ProjectCommands `json:"commands,omitempty"`
	Env      *EnvValue        `json:"env,omitempty"` // Project-level env files or variables
}

// ProjectCommands groups the user-provided commands at the project root.
type ProjectCommands struct {
	Up     string               `json:"up,omitempty"`
	Down   string               `json:"down,omitempty"`
	Health string               `json:"health,omitempty"`
	Dev    *EnvironmentCommands `json:"dev,omitempty"`
	Prod   *EnvironmentCommands `json:"prod,omitempty"`
}

// EnvironmentCommands holds dev/prod-specific command overrides.
type EnvironmentCommands struct {
	Up     string `json:"up,omitempty"`
	Down   string `json:"down,omitempty"`
	Health string `json:"health,omitempty"`
}

// EnvConfig describes how environment variables flow into the project.
type EnvConfig struct {
	UseGlobal bool              `json:"useGlobal"`
	Files     []string          `json:"files"`
	Variables map[string]string `json:"variables,omitempty"` // Direct variables to write to global.env
}
