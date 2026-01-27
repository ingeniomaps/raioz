package config

import (
	"fmt"
	"path/filepath"

	"raioz/internal/override"
)

// ApplyOverrides applies service overrides to the deps configuration
// Overrides take precedence over .raioz.json configuration
// Returns modified deps with overrides applied and list of overridden services
func ApplyOverrides(deps *Deps) (*Deps, []string, error) {
	overrides, err := override.LoadOverrides()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load overrides: %w", err)
	}

	// Clean invalid overrides first (best-effort, don't fail)
	_, _ = override.CleanInvalidOverrides()

	if len(overrides) == 0 {
		// No overrides, return original deps
		return deps, nil, nil
	}

	// Create a copy of deps to modify
	result := &Deps{
		SchemaVersion:      deps.SchemaVersion,
		Workspace:          deps.Workspace, // Preserve workspace
		Project:            deps.Project,
		Services:           make(map[string]Service),
		Infra:              deps.Infra, // Infra is not affected by overrides
		Env:                deps.Env,
		ProjectComposePath: deps.ProjectComposePath,
	}

	var appliedOverrides []string

	// Copy all services first
	for name, svc := range deps.Services {
		result.Services[name] = svc
	}

	// Apply overrides
	for serviceName, overrideConfig := range overrides {
		// Validate override path exists
		if err := override.ValidateOverridePath(overrideConfig.Path); err != nil {
			// Skip invalid overrides (already cleaned, but double-check)
			continue
		}

		// Check if service exists in deps
		originalSvc, exists := deps.Services[serviceName]
		if !exists {
			// Override for non-existent service - skip
			continue
		}

		// Only override git services (image services don't need path overrides)
		if originalSvc.Source.Kind != "git" {
			continue
		}

		// Resolve absolute path
		absPath, err := filepath.Abs(overrideConfig.Path)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to resolve override path for %s: %w", serviceName, err)
		}

		// Create new service with override applied
		// Keep all other configuration (ports, volumes, dependsOn, etc.)
		overrideSvc := originalSvc
		// Change Source.Path to point to override path
		// The path will be used as-is (absolute path) instead of workspace path
		overrideSvc.Source.Path = absPath

		// Replace service in result
		result.Services[serviceName] = overrideSvc
		appliedOverrides = append(appliedOverrides, serviceName)
	}

	return result, appliedOverrides, nil
}

// HasOverride checks if a service has an override
func HasOverride(serviceName string) (bool, error) {
	return override.HasOverride(serviceName)
}

// GetOverridePath returns the absolute path for a service override if it exists
// This is used during git operations and docker compose generation
func GetOverridePath(serviceName string) (string, bool, error) {
	overrideConfig, err := override.GetOverride(serviceName)
	if err != nil {
		return "", false, err
	}

	if overrideConfig == nil {
		return "", false, nil
	}

	// Validate path exists
	if err := override.ValidateOverridePath(overrideConfig.Path); err != nil {
		// Path doesn't exist, override is invalid
		return "", false, nil
	}

	absPath, err := filepath.Abs(overrideConfig.Path)
	if err != nil {
		return "", false, fmt.Errorf("failed to resolve override path: %w", err)
	}

	return absPath, true, nil
}
