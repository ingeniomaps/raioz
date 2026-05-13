package models

import (
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
	if e == nil {
		return fmt.Errorf("cannot unmarshal into nil EnvValue pointer")
	}

	var files []string
	if err := json.Unmarshal(data, &files); err == nil {
		e.Files = files
		e.Variables = nil
		e.IsObject = false
		return nil
	}

	var vars map[string]string
	if err := json.Unmarshal(data, &vars); err == nil {
		e.Files = nil
		e.Variables = vars
		e.IsObject = true
		return nil
	}

	return fmt.Errorf("env must be either an array of strings or an object with string values")
}

// MarshalJSON implements custom JSON marshaling for EnvValue
func (e EnvValue) MarshalJSON() ([]byte, error) {
	if e.IsObject && e.Variables != nil {
		data, err := json.Marshal(e.Variables)
		if err != nil {
			return nil, fmt.Errorf("marshal env variables: %w", err)
		}
		return data, nil
	}
	data, err := json.Marshal(e.Files)
	if err != nil {
		return nil, fmt.Errorf("marshal env files: %w", err)
	}
	return data, nil
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

	var name string
	if err := json.Unmarshal(data, &name); err == nil {
		n.Name = name
		n.Subnet = ""
		n.IsObject = false
		return nil
	}

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
		data, err := json.Marshal(n.Name)
		if err != nil {
			return nil, fmt.Errorf("marshal network name: %w", err)
		}
		return data, nil
	}
	data, err := json.Marshal(map[string]string{
		"name":   n.Name,
		"subnet": n.Subnet,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal network config: %w", err)
	}
	return data, nil
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
	Test          any    `json:"test,omitempty"`           // CMD: string or []string
	Interval      string `json:"interval,omitempty"`       // e.g. "30s"
	Timeout       string `json:"timeout,omitempty"`        // e.g. "10s"
	Retries       int    `json:"retries,omitempty"`        // e.g. 3
	StartPeriod   string `json:"start_period,omitempty"`   // e.g. "40s"
	StartInterval string `json:"start_interval,omitempty"` // e.g. "5s" (Compose v2.1+)
	Disable       bool   `json:"disable,omitempty"`        // if true, healthcheck is disabled
}
