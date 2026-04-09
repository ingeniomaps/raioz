package config

import (
	"raioz/internal/ignore"
)

// FilterIgnoredServices filters out services that are in the ignore list
// Returns filtered deps and list of ignored service names
func FilterIgnoredServices(deps *Deps) (*Deps, []string, error) {
	ignoredServices, err := ignore.GetIgnoredServices()
	if err != nil {
		return nil, nil, err
	}

	if len(ignoredServices) == 0 {
		// No ignored services, return original deps
		return deps, []string{}, nil
	}

	// Create set of ignored services for fast lookup
	ignoredSet := make(map[string]bool)
	for _, name := range ignoredServices {
		ignoredSet[name] = true
	}

	filtered := &Deps{
		SchemaVersion:      deps.SchemaVersion,
		Workspace:          deps.Workspace,
		Network:            deps.Network,
		Project:            deps.Project,
		Profiles:           deps.Profiles,
		Services:           make(map[string]Service),
		Infra:              deps.Infra,
		Env:                deps.Env,
		ProjectComposePath: deps.ProjectComposePath,
		Proxy:              deps.Proxy,
		ProxyConfig:        deps.ProxyConfig,
		PreHook:            deps.PreHook,
		PostHook:           deps.PostHook,
	}

	var ignored []string

	for name, svc := range deps.Services {
		if ignoredSet[name] {
			// Service is ignored, skip it
			ignored = append(ignored, name)
			continue
		}
		// Service is not ignored, include it
		filtered.Services[name] = svc
	}

	return filtered, ignored, nil
}

// CheckIgnoredDependencies checks if any services depend on ignored services
// Returns a map of service -> list of ignored dependencies
func CheckIgnoredDependencies(deps *Deps, ignoredServices []string) map[string][]string {
	if len(ignoredServices) == 0 {
		return nil
	}

	// Create set of ignored services for fast lookup
	ignoredSet := make(map[string]bool)
	for _, name := range ignoredServices {
		ignoredSet[name] = true
	}

	// Map of service -> ignored dependencies
	result := make(map[string][]string)

	for name, svc := range deps.Services {
		if ignoredSet[name] {
			continue // Skip ignored services themselves
		}

		// Skip if docker is nil (host execution - no docker dependencies)
		var ignoredDeps []string
		for _, dep := range svc.GetDependsOn() {
			if ignoredSet[dep] {
				ignoredDeps = append(ignoredDeps, dep)
			}
		}

		if len(ignoredDeps) > 0 {
			result[name] = ignoredDeps
		}
	}

	return result
}
