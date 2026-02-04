package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// MissingDependency represents a dependency that is required but not found
type MissingDependency struct {
	ServiceName string   // Service that requires the dependency
	RequiredBy  string   // Service that requires it (or "root" if from root config)
	Dependency  string   // Name of the missing dependency
	FoundConfig *Service // Config found in service's .raioz.json (if any)
	FoundPath   string   // Path where config was found (if any)
}

// DetectMissingDependencies detects dependencies that are required but not defined
// servicePathResolver is a function that returns the path to a service directory given service name and config
// This allows the function to work without importing workspace package
func DetectMissingDependencies(
	deps *Deps,
	servicePathResolver func(string, Service) string,
) ([]MissingDependency, error) {
	var missing []MissingDependency

	// Check each service for missing dependencies (service-level and docker-level)
	for name, svc := range deps.Services {
		for _, depName := range svc.GetDependsOn() {
			_, existsInServices := deps.Services[depName]
			_, existsInInfra := deps.Infra[depName]
			if !existsInServices && !existsInInfra {
				missing = append(missing, MissingDependency{
					ServiceName: depName,
					RequiredBy:  name,
					Dependency:  depName,
				})
			}
		}
	}

	// Try to find .raioz.json files in service directories to detect more dependencies
	// This is optional - we only search if the service path exists
	for name, svc := range deps.Services {
		if svc.Source.Kind != "git" {
			continue // Only check git services
		}

		// Check if service directory exists
		servicePath := servicePathResolver(name, svc)
		if _, err := os.Stat(servicePath); os.IsNotExist(err) {
			continue // Service directory doesn't exist yet, skip
		}

		// Try to find .raioz.json in service directory
		serviceConfigPath := filepath.Join(servicePath, ".raioz.json")
		if _, err := os.Stat(serviceConfigPath); os.IsNotExist(err) {
			continue // No .raioz.json in service directory
		}

		// Try to load service's .raioz.json
		serviceDeps, _, err := LoadDeps(serviceConfigPath)
		if err != nil {
			// Failed to load, skip (non-fatal)
			continue
		}

		// Check dependencies in service's .raioz.json
		for depName, depSvc := range serviceDeps.Services {
			// Check if dependency is already in root config
			if _, exists := deps.Services[depName]; !exists {
				// Dependency found in service config but not in root
				missing = append(missing, MissingDependency{
					ServiceName: depName,
					RequiredBy:  name,
					Dependency:  depName,
					FoundConfig: &depSvc,
					FoundPath:   serviceConfigPath,
				})
			}
		}
	}

	return missing, nil
}

// FindServiceConfig finds a .raioz.json file for a service
// Returns the path and loaded config if found
func FindServiceConfig(servicePath string) (*Deps, string, error) {
	configPath := filepath.Join(servicePath, ".raioz.json")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, "", fmt.Errorf(".raioz.json not found in service directory")
	}

	deps, _, err := LoadDeps(configPath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to load service config: %w", err)
	}

	return deps, configPath, nil
}

// DependencyConflict represents a conflict between root and service dependencies
type DependencyConflict struct {
	ServiceName   string   // Service name
	RootConfig    *Service // Config in root .raioz.json
	ServiceConfig *Service // Config in service's .raioz.json
	Differences   []string // List of differences
}

// DetectDependencyConflicts detects conflicts between root and service dependencies
// servicePathResolver is a function that returns the path to a service directory given service name and config
func DetectDependencyConflicts(
	deps *Deps,
	servicePathResolver func(string, Service) string,
) ([]DependencyConflict, error) {
	var conflicts []DependencyConflict

	// Check each service for conflicts
	for name, rootSvc := range deps.Services {
		if rootSvc.Source.Kind != "git" {
			continue // Only check git services
		}

		// Check if service directory exists
		servicePath := servicePathResolver(name, rootSvc)
		if _, err := os.Stat(servicePath); os.IsNotExist(err) {
			continue // Service directory doesn't exist yet, skip
		}

		// Try to find .raioz.json in service directory
		serviceDeps, _, err := FindServiceConfig(servicePath)
		if err != nil {
			// No service config found, skip
			continue
		}

		// Check if service is defined in service's .raioz.json
		serviceSvc, exists := serviceDeps.Services[name]
		if !exists {
			continue // Service not defined in its own .raioz.json
		}

		// Compare root config with service config
		var differences []string

		// Compare source branch
		if rootSvc.Source.Branch != serviceSvc.Source.Branch {
			differences = append(differences, fmt.Sprintf(
				"branch: root uses '%s', service uses '%s'",
				rootSvc.Source.Branch, serviceSvc.Source.Branch,
			))
		}

		// Compare source repo (if different)
		if rootSvc.Source.Repo != serviceSvc.Source.Repo {
			differences = append(differences, fmt.Sprintf(
				"repo: root uses '%s', service uses '%s'",
				rootSvc.Source.Repo, serviceSvc.Source.Repo,
			))
		}

		// Compare docker mode (check if both have docker config)
		if rootSvc.Docker != nil && serviceSvc.Docker != nil {
			if rootSvc.Docker.Mode != serviceSvc.Docker.Mode {
				differences = append(differences, fmt.Sprintf(
					"mode: root uses '%s', service uses '%s'",
					rootSvc.Docker.Mode, serviceSvc.Docker.Mode,
				))
			}

			// Compare ports (simplified comparison)
			if !equalStringSlices(rootSvc.Docker.Ports, serviceSvc.Docker.Ports) {
				differences = append(differences, "ports: different")
			}
		} else if rootSvc.Docker != nil || serviceSvc.Docker != nil {
			// One has docker config, the other doesn't
			differences = append(differences, "docker: one has docker config, the other doesn't")
		}

		if len(differences) > 0 {
			conflicts = append(conflicts, DependencyConflict{
				ServiceName:   name,
				RootConfig:    &rootSvc,
				ServiceConfig: &serviceSvc,
				Differences:   differences,
			})
		}
	}

	return conflicts, nil
}

// equalStringSlices compares two string slices for equality
func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
