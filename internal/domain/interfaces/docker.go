package interfaces

// ServiceInfo represents information about a Docker service
type ServiceInfo struct {
	Status   string
	Uptime   string
	CPU      string
	Memory   string
	Image    string
	Commit   string
	Branch   string
	Health   string
	Restarts string
	Ports    []string
}

// LogsOptions contains options for viewing service logs
type LogsOptions struct {
	Follow   bool
	Tail     int
	Services []string
}

// DockerRunner is the aggregate port covering every Docker-shaped
// operation raioz performs. ADR-012 splits the surface
// into six segregated interfaces — ContainerManager, ComposeRunner,
// NetworkManager, VolumeManager, ImageValidator, PortValidator — that
// this interface embeds. Aggregating callers stay on this type; new
// callers should reference the smallest segregated interface they
// actually use so tests can mock narrowly.
//
// A handful of methods that don't fit a single segregated interface
// (presenters that pre-date the split, naming helpers) remain on
// DockerRunner for now. They are slated for migration to
// internal/output/ and internal/naming/ in follow-up work; see
// ADR-012's "Implementation status" section.
type DockerRunner interface {
	ContainerManager
	ComposeRunner
	NetworkManager
	VolumeManager
	ImageValidator
	PortValidator

	// FormatStatusTable formats service information as a table.
	//
	// Deprecated: presentation belongs in internal/output/. Use
	// `output.PrintStatusTable` directly — this method delegates to it
	// and will be removed once every caller migrates.
	FormatStatusTable(services map[string]*ServiceInfo, jsonOutput bool) error
	// FormatPortConflicts formats port conflicts for display.
	//
	// Deprecated: use `output.FormatPortConflicts`.
	FormatPortConflicts(conflicts []PortConflict) string
	// FormatSharedVolumesWarning formats a warning when multiple
	// services share a named volume (raioz's heuristic spots this and
	// surfaces the risk before they collide at runtime).
	//
	// Deprecated: use `output.FormatSharedVolumesWarning`.
	FormatSharedVolumesWarning(sharedVolumes map[string][]string) string

	// NormalizeVolumeName, NormalizeContainerName, NormalizeInfraName
	// are naming-policy helpers, not Docker operations. They live here
	// for compatibility; see internal/naming/ for the canonical home.
	//
	// Deprecated: use `naming.NormalizeVolumeName` etc.
	NormalizeVolumeName(prefix, name string) (string, error)
	// Deprecated: use `naming.NormalizeContainerName`.
	NormalizeContainerName(
		workspace, service, project string, hasExplicitWorkspace bool,
	) (string, error)
	// Deprecated: use `naming.NormalizeInfraName`.
	NormalizeInfraName(
		workspace, infra, project string, hasExplicitWorkspace bool,
	) (string, error)
}

// PortInfo represents information about an active port
type PortInfo struct {
	Port          string
	Project       string
	Service       string
	HostPort      int
	ContainerPort int
}

// PortConflict represents a port conflict with another project
type PortConflict struct {
	Port        string
	Project     string
	Service     string
	Alternative string
}

// ServiceVolumes represents the volumes used by a service
type ServiceVolumes struct {
	NamedVolumes []string
}
