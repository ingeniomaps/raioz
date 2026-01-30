package config

import (
	"fmt"
)

// FilterByFeatureFlags filters services based on feature flags and profile
// Returns filtered deps and a list of services that were replaced with mocks
func FilterByFeatureFlags(deps *Deps, profile string, envVars map[string]string) (*Deps, []string) {
	filtered := &Deps{
		SchemaVersion:      deps.SchemaVersion,
		Workspace:          deps.Workspace, // Preserve workspace
		Network:            deps.Network,   // Preserve network
		Project:            deps.Project,
		Services:           make(map[string]Service),
		Infra:              deps.Infra, // Infra is always included
		Env:                deps.Env,
		ProjectComposePath: deps.ProjectComposePath,
	}

	var mockServices []string

	for name, svc := range deps.Services {
		// Check explicit enabled field first (takes precedence)
		if svc.Enabled != nil && !*svc.Enabled {
			continue // Skip explicitly disabled service
		}

		// Check if service should be enabled (feature flags)
		if !ShouldEnableService(svc, profile, envVars) {
			continue // Skip disabled service
		}

		// Check if we should use mock
		if ShouldUseMock(svc, profile, envVars) {
			// Replace service with mock configuration
			var mockEnv *EnvValue
			if len(svc.Mock.Env) > 0 {
				// Convert []string to EnvValue with file paths
				mockEnv = &EnvValue{
					Files:     svc.Mock.Env,
					Variables: nil,
					IsObject:  false,
				}
			}
			mockSvc := Service{
				Source: SourceConfig{
					Kind:  "image",
					Image: svc.Mock.Image,
					Tag:   svc.Mock.Tag,
				},
				Docker: &DockerConfig{
					Mode:  svc.Docker.Mode,
					Ports: svc.Mock.Ports,
				},
				Env:      mockEnv,
				Profiles: svc.Profiles,
			}
			// Use mock ports if provided, otherwise use original
			if len(mockSvc.Docker.Ports) == 0 {
				mockSvc.Docker.Ports = svc.Docker.Ports
			}
			// Preserve dependsOn from original service
			mockSvc.Docker.DependsOn = svc.Docker.DependsOn
			// Preserve volumes if any
			mockSvc.Docker.Volumes = svc.Docker.Volumes

			filtered.Services[name] = mockSvc
			mockServices = append(mockServices, name)
		} else {
			// Use original service
			filtered.Services[name] = svc
		}
	}

	return filtered, mockServices
}

// ValidateFeatureFlags validates feature flag configurations
func ValidateFeatureFlags(deps *Deps) error {
	for name, svc := range deps.Services {
		// Validate mock configuration
		if svc.Mock != nil && svc.Mock.Enabled {
			if svc.Mock.Image == "" {
				return fmt.Errorf(
					"service %s: mock enabled but no image specified",
					name,
				)
			}
		}

		// Validate feature flag configuration
		if svc.FeatureFlag != nil {
			// If both enabled and disabled are true, disabled wins
			if svc.FeatureFlag.Enabled && svc.FeatureFlag.Disabled {
				return fmt.Errorf(
					"service %s: featureFlag cannot be both enabled and disabled",
					name,
				)
			}

			// Validate profile values
			for _, p := range svc.FeatureFlag.Profiles {
				if p != "frontend" && p != "backend" {
					return fmt.Errorf(
						"service %s: invalid profile '%s' in featureFlag (must be 'frontend' or 'backend')",
						name, p,
					)
				}
			}
		}
	}

	return nil
}
