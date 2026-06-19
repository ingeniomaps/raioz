package models

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// Infra represents an inline infrastructure definition.
type Infra struct {
	// Name is an optional literal container-name override. Empty means raioz
	// picks the name based on workspace/project (see naming.SharedContainer
	// / naming.Container).
	Name string `json:"name,omitempty"`

	// Compose is the list of user-supplied docker-compose fragment paths
	// that define this dependency. Mutually exclusive with Image. When
	// set, raioz runs `docker compose -f <file> ... -f <overlay> up -d`
	// with the user's files PLUS a raioz-generated overlay that adds the
	// workspace network and stamps raioz labels on every service. The
	// user's file controls image, volumes, healthchecks, ports, etc.;
	// raioz only wires it up to the shared network + labels.
	Compose []string `json:"compose,omitempty"`

	Image string `json:"image,omitempty"`
	Tag   string `json:"tag,omitempty"`
	// Ports is the legacy publish list. Kept for backwards compatibility;
	// new configs should use Expose + Publish (see fields below).
	Ports       []string           `json:"ports,omitempty"`
	Volumes     []string           `json:"volumes,omitempty"`     // Optional: can be null or empty array
	Env         *EnvValue          `json:"env,omitempty"`         // File paths or variables
	IP          string             `json:"ip,omitempty"`          // Static IP (e.g. "150.150.0.10")
	Healthcheck *HealthcheckConfig `json:"healthcheck,omitempty"` // Optional: same format as Docker Compose healthcheck
	Profiles    []string           `json:"profiles,omitempty"`    // Profile filter for --profile
	Seed        []string           `json:"seed,omitempty"`        // Files for /docker-entrypoint-initdb.d/

	// Expose declares the container ports this dependency listens on. Used
	// by discovery/proxy to build correct URLs and by the publish allocator
	// to decide what container port to map when Publish.Auto is set. Zero
	// length means "raioz doesn't know; best effort".
	Expose []int `json:"expose,omitempty"`

	// Publish is the opt-in host-side binding. nil means internal-only (the
	// dep lives on the Docker network; containers reach it by DNS, host
	// tools do not). See PublishSpec for the semantic fields.
	Publish *PublishSpec `json:"publish,omitempty"`

	// Routing overrides the proxy behavior for this dependency. Setting it
	// (even to an empty object) opts the dep into getting an HTTPS route,
	// which is the escape hatch for images whose name matches the DB/broker
	// heuristic (e.g. a bespoke "postgres-admin-ui" container that does
	// actually speak HTTP). See internal/app/upcase/orchestration_proxy.go.
	Routing *RoutingConfig `json:"routing,omitempty"`

	// ProxyOverride forces a specific (target, port) pair for the proxy,
	// bypassing detection. Mirrors Service.ProxyOverride and is populated
	// from the user's `dependencies.<name>.proxy:` block in raioz.yaml.
	ProxyOverride *ServiceProxyOverride `json:"proxyOverride,omitempty"`

	// Hostname overrides the proxy subdomain for this dependency. Empty
	// means "use the entry name". Mirrors Service.Hostname so a dep can
	// be reached at https://<hostname>.<domain> instead of the default
	// https://<entry-name>.<domain>.
	Hostname string `json:"hostname,omitempty"`

	// HostnameAliases exposes the same dep under extra subdomains.
	// Mirrors Service.HostnameAliases. Empty means the dep is only
	// reachable through Hostname (or the entry name when Hostname is
	// empty).
	HostnameAliases []string `json:"hostnameAliases,omitempty"`

	// Project is the path to a sibling raioz project that owns this dep
	// (mode A of ADR-008). When set, raioz brings the sibling up via
	// recursive `raioz up` in its cwd if it's not already running, and
	// never touches it on `raioz down`. Empty means "this is a normal
	// image/compose dep".
	//
	// Resolved to an absolute path by yaml_loader.go::resolveYAMLPaths
	// before the bridge runs.
	Project string `json:"project,omitempty"`

	// SiblingProject is the mode-B fallback marker (ADR-008): when set
	// alongside Image/Compose, raioz skips the image-based dispatch only
	// when the sibling project is currently active; otherwise the image
	// is started normally. Mutually exclusive with Project.
	SiblingProject string `json:"siblingProject,omitempty"`

	// RequiredHostname is the optional hostname assertion when Project
	// or SiblingProject is set. Empty means "no validation".
	RequiredHostname string `json:"requiredHostname,omitempty"`
}

// ServiceProxyOverride tells the proxy exactly where to reverse_proxy for a
// service, overriding auto-detection. Populated from the user's
// `services.<name>.proxy:` block in raioz.yaml.
type ServiceProxyOverride struct {
	// Disabled opts the service out of getting a proxy route at all
	// (`proxy: false` in raioz.yaml). When true, raioz creates no
	// https://<name>.<domain> route — used for host-net services with no UI.
	Disabled bool   `json:"disabled,omitempty"`
	Target   string `json:"target,omitempty"`
	Port     int    `json:"port,omitempty"`
}

// PublishSpec captures the user's host-side binding intent for a dependency.
// Populated by the YAML bridge from YAMLPublish and consumed by the port
// allocator and ImageRunner.
type PublishSpec struct {
	// Auto asks raioz to pick a free host port automatically. Mutually
	// exclusive with Ports.
	Auto bool `json:"auto,omitempty"`
	// Ports are the explicit host ports the user requested. Each entry is
	// mapped to the matching container port from Infra.Expose at the same
	// index, or to the same port number when Expose is empty/shorter.
	Ports []int `json:"ports,omitempty"`
}

// InfraEntry is a single infra entry: either a YAML file path or an inline spec.
// In JSON, the value can be a string (path) or an object (inline).
type InfraEntry struct {
	Path   string `json:"-"` // Path to Docker Compose YAML fragment
	Inline *Infra `json:"-"` // Inline infra spec (mutually exclusive with Path)
}

// Profiles returns the profiles for this entry (only for inline; path-based entries have no profile filter).
func (e *InfraEntry) Profiles() []string {
	if e.Inline != nil {
		return e.Inline.Profiles
	}
	return nil
}

// UnmarshalJSON allows infra entry to be either a string (path to YAML) or an object (inline spec).
func (e *InfraEntry) UnmarshalJSON(data []byte) error {
	if e == nil {
		return fmt.Errorf("cannot unmarshal into nil InfraEntry pointer")
	}
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) >= 2 && trimmed[0] == '"' && trimmed[len(trimmed)-1] == '"' {
		var path string
		if err := json.Unmarshal(data, &path); err != nil {
			return fmt.Errorf("unmarshal infra path: %w", err)
		}
		e.Path = path
		e.Inline = nil
		return nil
	}
	var inf Infra
	if err := json.Unmarshal(data, &inf); err != nil {
		return fmt.Errorf("unmarshal inline infra: %w", err)
	}
	e.Path = ""
	e.Inline = &inf
	return nil
}

// MarshalJSON emits either the path string or the inline object.
func (e InfraEntry) MarshalJSON() ([]byte, error) {
	if e.Path != "" {
		out, err := json.Marshal(e.Path)
		if err != nil {
			return nil, fmt.Errorf("marshal infra path: %w", err)
		}
		return out, nil
	}
	if e.Inline != nil {
		out, err := json.Marshal(e.Inline)
		if err != nil {
			return nil, fmt.Errorf("marshal inline infra: %w", err)
		}
		return out, nil
	}
	return []byte("null"), nil
}
