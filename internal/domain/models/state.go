package models

import (
	"slices"
	"time"
)

// LocalState is the minimal state file stored in the project directory.
// Docker is the source of truth for running state; this file only stores
// what Docker can't tell us: dev overrides, ignored services, host PIDs.
type LocalState struct {
	Project      string                 `json:"project"`
	Workspace    string                 `json:"workspace,omitempty"`
	LastUp       time.Time              `json:"lastUp"`
	DevOverrides map[string]DevOverride `json:"devOverrides,omitempty"`
	Ignored      []string               `json:"ignored,omitempty"`
	HostPIDs     map[string]int         `json:"hostPIDs,omitempty"`
	NetworkName  string                 `json:"networkName,omitempty"`

	// DeferredToSibling lists the dep names whose dispatch was skipped
	// at `up` time because a sibling raioz project was already serving
	// them (ADR-008 mode B). `down` consults this list so it doesn't
	// try to tear down containers the consumer never created. The list
	// is rewritten on every `up` — entries persist only as long as the
	// sibling stays active.
	DeferredToSibling []string `json:"deferredToSibling,omitempty"`

	// ProjectComposePath is the docker-compose.yml path that `raioz up`
	// detected (or that the user pointed at) for the project. Captured
	// at up-time and consumed by inspection commands (`logs`, `exec`,
	// `restart`) so they don't need to redetect or fall back to the
	// legacy whole-Deps snapshot. Empty when no compose file was used
	// (pure host services). Added in ADR-011 Phase 2.
	ProjectComposePath string `json:"projectComposePath,omitempty"`

	// ProjectRoot is the absolute path to the directory that holds
	// `raioz.yaml` for this project. Captured at up-time so commands
	// invoked from anywhere can locate the project source without
	// requiring the user to be in the project directory. Added in
	// ADR-011 Phase 2.
	ProjectRoot string `json:"projectRoot,omitempty"`
}

// DevOverride records that a dependency has been promoted to local development.
type DevOverride struct {
	OriginalImage string    `json:"originalImage"`
	LocalPath     string    `json:"localPath"`
	PromotedAt    time.Time `json:"promotedAt"`
}

// AddDevOverride records a dependency being promoted to local.
func (s *LocalState) AddDevOverride(name, originalImage, localPath string) {
	s.DevOverrides[name] = DevOverride{
		OriginalImage: originalImage,
		LocalPath:     localPath,
		PromotedAt:    time.Now(),
	}
}

// RemoveDevOverride removes a dev override.
func (s *LocalState) RemoveDevOverride(name string) {
	delete(s.DevOverrides, name)
}

// IsDevOverridden returns true if a dependency is currently in dev mode.
func (s *LocalState) IsDevOverridden(name string) bool {
	_, ok := s.DevOverrides[name]
	return ok
}

// GetDevOverride returns the dev override for a dependency, if any.
func (s *LocalState) GetDevOverride(name string) (DevOverride, bool) {
	o, ok := s.DevOverrides[name]
	return o, ok
}

// MarkDeferred records that a dep was skipped at `up` time because a
// sibling raioz project was already serving it (ADR-008 mode B). The
// matching `down` reads this list to skip the dep too — without it,
// raioz would try to tear down a container it never created. No-op if
// the dep is already on the list, so callers can call this without
// guarding for the second `up` in a row.
func (s *LocalState) MarkDeferred(name string) {
	if s.IsDeferred(name) {
		return
	}
	s.DeferredToSibling = append(s.DeferredToSibling, name)
}

// ClearDeferred removes a dep from the deferred list. Called by `up`
// when the sibling is no longer active and the local fallback needs to
// start.
func (s *LocalState) ClearDeferred(name string) {
	s.DeferredToSibling = slices.DeleteFunc(
		s.DeferredToSibling, func(d string) bool { return d == name })
}

// IsDeferred reports whether the most recent `up` recorded this dep as
// deferred to a sibling project.
func (s *LocalState) IsDeferred(name string) bool {
	return slices.Contains(s.DeferredToSibling, name)
}

// GlobalState represents the global state across all projects.
type GlobalState struct {
	ActiveProjects []string                `json:"activeProjects"`
	Projects       map[string]ProjectState `json:"projects"`
}

// ProjectState represents the state of a single project.
type ProjectState struct {
	Name          string         `json:"name"`
	Workspace     string         `json:"workspace"`
	LastExecution time.Time      `json:"lastExecution"`
	Services      []ServiceState `json:"services"`
}

// ServiceState represents the state of a single service.
type ServiceState struct {
	Name    string `json:"name"`
	Mode    string `json:"mode"`            // dev or prod
	Version string `json:"version"`         // Commit SHA or image tag
	Image   string `json:"image,omitempty"` // Full image name (if applicable)
	Status  string `json:"status"`          // running or stopped
}

// ServiceInfo is a minimal interface for service information.
// This avoids circular dependency with docker package.
type ServiceInfo struct {
	Status  string
	Version string
	Image   string
}

// ConfigChange represents a detected change in configuration.
type ConfigChange struct {
	Type     string // "service", "infra", "project"
	Name     string // Service/infra name
	Field    string // Field that changed
	OldValue string
	NewValue string
}

// AlignmentIssue represents a detected alignment issue.
type AlignmentIssue struct {
	Type        string // "branch_drift", "config_change", "port_conflict", "env_change"
	Severity    string // "info", "warning", "critical"
	Service     string // Service name (if applicable)
	Description string // Human-readable description
	Suggestion  string // Suggested action
}

// ServicePreference represents a user's preference for handling service conflicts.
type ServicePreference struct {
	ServiceName string    `json:"serviceName"`           // Name of the service (e.g., "nginx")
	Preference  string    `json:"preference"`            // "local" | "cloned" | "ask"
	ProjectPath string    `json:"projectPath,omitempty"` // Path to local project (if preference is "local")
	Workspace   string    `json:"workspace,omitempty"`   // Workspace name (if preference is "cloned")
	Reason      string    `json:"reason,omitempty"`      // Reason for the preference
	Timestamp   time.Time `json:"timestamp"`
}

// ServicePreferences represents all service preferences.
type ServicePreferences struct {
	Preferences map[string]ServicePreference `json:"preferences"` // Key: serviceName
}

// WorkspaceProjectPreference stores which project to use when multiple
// .raioz.json in the same workspace define overlapping services
// (e.g. same service name).
type WorkspaceProjectPreference struct {
	PreferredProject string `json:"preferredProject"` // project name to use when conflict
	AlwaysAsk        bool   `json:"alwaysAsk"`        // if true, always prompt instead of applying preference
	// if true and preferred project matches, merge configs
	MergeWhenPreferred bool `json:"mergeWhenPreferred"`
}

// WorkspacePreferences is the file format: workspace name -> preference.
type WorkspacePreferences struct {
	ByWorkspace map[string]WorkspaceProjectPreference `json:"byWorkspace"`
}
