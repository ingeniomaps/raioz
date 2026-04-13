package config

import (
	"bytes"
	"encoding/json"
	"fmt"
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
	Name     string // Network name (always present)
	Subnet   string // Optional subnet (CIDR notation, e.g., "150.150.0.0/16")
	IsObject bool   // True if this was deserialized as an object
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

	return fmt.Errorf(
		"network must be a string (name) or object with 'name' and optional 'subnet'",
	)
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

// HealthcheckConfig matches Docker Compose healthcheck format.
// See https://docs.docker.com/compose/compose-file/05-services/#healthcheck
type HealthcheckConfig struct {
	Test          interface{} `json:"test,omitempty"`           // CMD: string or []string
	Interval      string      `json:"interval,omitempty"`       // e.g. "30s"
	Timeout       string      `json:"timeout,omitempty"`        // e.g. "10s"
	Retries       int         `json:"retries,omitempty"`        // e.g. 3
	StartPeriod   string      `json:"start_period,omitempty"`   // e.g. "40s"
	StartInterval string      `json:"start_interval,omitempty"` // e.g. "5s" (Compose v2.1+)
	Disable       bool        `json:"disable,omitempty"`        // if true, healthcheck is disabled
}

// Infra represents an inline infrastructure definition.
type Infra struct {
	Image string `json:"image"`
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
			return err
		}
		e.Path = path
		e.Inline = nil
		return nil
	}
	var inf Infra
	if err := json.Unmarshal(data, &inf); err != nil {
		return err
	}
	e.Path = ""
	e.Inline = &inf
	return nil
}

// MarshalJSON emits either the path string or the inline object.
func (e InfraEntry) MarshalJSON() ([]byte, error) {
	if e.Path != "" {
		return json.Marshal(e.Path)
	}
	if e.Inline != nil {
		return json.Marshal(e.Inline)
	}
	return []byte("null"), nil
}
