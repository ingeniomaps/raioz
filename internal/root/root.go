package root

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"raioz/internal/config"
	"raioz/internal/workspace"
)

const rootFileName = "raioz.root.json"

// ServiceOrigin represents the origin of a service configuration
type ServiceOrigin string

const (
	OriginRoot    ServiceOrigin = "root"    // From .raioz.json (original)
	OriginOverride ServiceOrigin = "override" // From override system
	OriginAssisted ServiceOrigin = "assisted" // Added via dependency assist
)

// ServiceMetadata contains metadata about a service in the root configuration
type ServiceMetadata struct {
	Origin      ServiceOrigin `json:"origin"`
	AddedAt     string        `json:"addedAt,omitempty"`     // ISO 8601 timestamp
	AddedBy     string        `json:"addedBy,omitempty"`     // User or system action
	PreviousOrigin ServiceOrigin `json:"previousOrigin,omitempty"` // For overrides
	Reason      string        `json:"reason,omitempty"`      // Reason for override/assist
}

// RootConfig represents the resolved root configuration for a workspace
type RootConfig struct {
	SchemaVersion string                    `json:"schemaVersion"`
	GeneratedAt   string                    `json:"generatedAt"`   // ISO 8601 timestamp
	LastUpdatedAt string                    `json:"lastUpdatedAt"` // ISO 8601 timestamp
	Project       config.Project            `json:"project"`
	Services      map[string]config.Service `json:"services"`
	Infra         map[string]config.Infra   `json:"infra"`
	Env           config.EnvConfig          `json:"env"`
	// Metadata tracks the origin of each service
	Metadata map[string]ServiceMetadata `json:"metadata,omitempty"`
}

// GetRootPath returns the path to the raioz.root.json file for a workspace
func GetRootPath(ws *workspace.Workspace) string {
	return filepath.Join(ws.Root, rootFileName)
}

// Exists checks if raioz.root.json exists for a workspace
func Exists(ws *workspace.Workspace) bool {
	path := GetRootPath(ws)
	_, err := os.Stat(path)
	return err == nil
}

// Load loads raioz.root.json from a workspace
func Load(ws *workspace.Workspace) (*RootConfig, error) {
	path := GetRootPath(ws)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // File doesn't exist, return nil
		}
		return nil, fmt.Errorf("failed to read root config: %w", err)
	}

	var root RootConfig
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("failed to unmarshal root config: %w", err)
	}

	// Initialize metadata map if nil
	if root.Metadata == nil {
		root.Metadata = make(map[string]ServiceMetadata)
	}

	return &root, nil
}

// Save saves raioz.root.json to a workspace
func Save(ws *workspace.Workspace, root *RootConfig) error {
	path := GetRootPath(ws)

	// Set timestamps if not set
	now := time.Now().UTC().Format(time.RFC3339)
	if root.GeneratedAt == "" {
		root.GeneratedAt = now
	}
	root.LastUpdatedAt = now

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create root config directory: %w", err)
	}

	data, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal root config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write root config: %w", err)
	}

	return nil
}

// GenerateFromDeps generates a RootConfig from a Deps configuration
// This is used when creating a new root config from .raioz.json
// assistedServices is a map of service names to their "addedBy" identifier for dependency assist
func GenerateFromDeps(deps *config.Deps, appliedOverrides []string, assistedServices map[string]string) (*RootConfig, error) {
	now := time.Now().UTC().Format(time.RFC3339)

	root := &RootConfig{
		SchemaVersion: deps.SchemaVersion,
		GeneratedAt:   now,
		LastUpdatedAt: now,
		Project:       deps.Project,
		Services:      make(map[string]config.Service),
		Infra:         deps.Infra,
		Env:           deps.Env,
		Metadata:      make(map[string]ServiceMetadata),
	}

	// Copy services
	for name, svc := range deps.Services {
		root.Services[name] = svc

		// Set metadata based on whether service has override
		isOverride := false
		for _, overrideName := range appliedOverrides {
			if overrideName == name {
				isOverride = true
				break
			}
		}

		if isOverride {
			root.Metadata[name] = ServiceMetadata{
				Origin: OriginOverride,
				AddedAt: now,
				AddedBy: "override",
			}
		} else {
			root.Metadata[name] = ServiceMetadata{
				Origin: OriginRoot,
				AddedAt: now,
				AddedBy: ".raioz.json",
			}
		}
	}

	return root, nil
}

// UpdateFromDeps updates an existing RootConfig from a Deps configuration
// This preserves existing metadata and updates services
// assistedServices is a map of service names to their "addedBy" identifier for dependency assist
func UpdateFromDeps(root *RootConfig, deps *config.Deps, appliedOverrides []string, assistedServices map[string]string) error {
	now := time.Now().UTC().Format(time.RFC3339)

	// Update basic fields
	root.Project = deps.Project
	root.Infra = deps.Infra
	root.Env = deps.Env
	root.LastUpdatedAt = now

	// Track which services are overrides
	overrideSet := make(map[string]bool)
	for _, overrideName := range appliedOverrides {
		overrideSet[overrideName] = true
	}

	// Update services and metadata
	existingServices := make(map[string]bool)
	for name, svc := range deps.Services {
		existingServices[name] = true
		root.Services[name] = svc

		// Update metadata
		if meta, exists := root.Metadata[name]; exists {
			// Service already exists, check if origin changed
			if overrideSet[name] && meta.Origin != OriginOverride {
				// Service was overridden
				root.Metadata[name] = ServiceMetadata{
					Origin:         OriginOverride,
					AddedAt:        meta.AddedAt, // Preserve original added date
					AddedBy:        "override",
					PreviousOrigin: meta.Origin,
					Reason:         "manual override",
				}
			}
			// Otherwise, preserve existing metadata
		} else {
			// New service
			if overrideSet[name] {
				root.Metadata[name] = ServiceMetadata{
					Origin:  OriginOverride,
					AddedAt: now,
					AddedBy: "override",
				}
			} else if addedBy, isAssisted := assistedServices[name]; isAssisted {
				// Service was added via dependency assist
				root.Metadata[name] = ServiceMetadata{
					Origin:  OriginAssisted,
					AddedAt: now,
					AddedBy: addedBy,
					Reason:  fmt.Sprintf("dependency assist: required by %s", addedBy),
				}
			} else {
				root.Metadata[name] = ServiceMetadata{
					Origin:  OriginRoot,
					AddedAt: now,
					AddedBy: ".raioz.json",
				}
			}
		}
	}

	// Remove services that no longer exist in deps
	for name := range root.Services {
		if !existingServices[name] {
			delete(root.Services, name)
			delete(root.Metadata, name)
		}
	}

	return nil
}

// ToDeps converts a RootConfig back to a Deps configuration
// This is used when loading root config as the source of truth
func (r *RootConfig) ToDeps() *config.Deps {
	return &config.Deps{
		SchemaVersion: r.SchemaVersion,
		Project:       r.Project,
		Services:      r.Services,
		Infra:         r.Infra,
		Env:           r.Env,
	}
}

// AddAssistedService adds a service that was added via dependency assist
func (r *RootConfig) AddAssistedService(name string, svc config.Service, addedBy string, reason string) {
	now := time.Now().UTC().Format(time.RFC3339)
	r.Services[name] = svc
	r.Metadata[name] = ServiceMetadata{
		Origin:  OriginAssisted,
		AddedAt: now,
		AddedBy: addedBy,
		Reason:  reason,
	}
	r.LastUpdatedAt = now
}
