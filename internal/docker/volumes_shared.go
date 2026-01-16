package docker

import (
	"fmt"
	"sort"

	"raioz/internal/config"
)

// SharedVolume represents a volume that is shared between multiple services
type SharedVolume struct {
	VolumeName string
	Services   []string // List of service names that use this volume
}

// DetectSharedVolumes detects volumes that are shared between multiple services
// Returns a map of volume name to list of services that use it
func DetectSharedVolumes(services map[string]ServiceVolumes) map[string][]string {
	volumeToServices := make(map[string][]string)

	// Collect all volumes and which services use them
	for serviceName, svcVolumes := range services {
		for _, volName := range svcVolumes.NamedVolumes {
			volumeToServices[volName] = append(volumeToServices[volName], serviceName)
		}
	}

	// Filter to only shared volumes (used by 2+ services)
	shared := make(map[string][]string)
	for volName, serviceList := range volumeToServices {
		if len(serviceList) > 1 {
			// Sort service names for consistent output
			sort.Strings(serviceList)
			shared[volName] = serviceList
		}
	}

	return shared
}

// ServiceVolumes represents the volumes used by a service
type ServiceVolumes struct {
	NamedVolumes []string // Only named volumes (not bind mounts)
}

// FormatSharedVolumesWarning formats a warning message for shared volumes
func FormatSharedVolumesWarning(sharedVolumes map[string][]string) string {
	if len(sharedVolumes) == 0 {
		return ""
	}

	var result string
	result += fmt.Sprintf("\n⚠️  Warning: %d volume(s) are shared between multiple services:\n\n", len(sharedVolumes))

	// Sort volume names for consistent output
	volNames := make([]string, 0, len(sharedVolumes))
	for volName := range sharedVolumes {
		volNames = append(volNames, volName)
	}
	sort.Strings(volNames)

	for _, volName := range volNames {
		services := sharedVolumes[volName]
		result += fmt.Sprintf("  • Volume '%s' is used by: %v\n", volName, services)
	}

	result += "\n"
	result += "  ℹ️  Note: Shared volumes can be intentional (e.g., shared database data).\n"
	result += "     Ensure that services are designed to handle concurrent access to shared volumes.\n"

	return result
}

// BuildServiceVolumesMap builds a map of service volumes from Deps configuration
// This extracts named volumes from both services and infra
func BuildServiceVolumesMap(deps *config.Deps) (map[string]ServiceVolumes, error) {
	serviceVolumes := make(map[string]ServiceVolumes)

	// Process services
	for serviceName, svc := range deps.Services {
		// Skip if docker is nil (host execution - no docker volumes)
		if svc.Docker == nil {
			continue
		}

		// Extract named volumes from service
		namedVols, err := ExtractNamedVolumes(svc.Docker.Volumes)
		if err != nil {
			return nil, fmt.Errorf("failed to extract named volumes for service %s: %w", serviceName, err)
		}

		if len(namedVols) > 0 {
			serviceVolumes[serviceName] = ServiceVolumes{
				NamedVolumes: namedVols,
			}
		}
	}

	// Process infra (infra services can also share volumes)
	for infraName, infra := range deps.Infra {
		// Extract named volumes from infra
		namedVols, err := ExtractNamedVolumes(infra.Volumes)
		if err != nil {
			return nil, fmt.Errorf("failed to extract named volumes for infra %s: %w", infraName, err)
		}

		if len(namedVols) > 0 {
			serviceVolumes[infraName] = ServiceVolumes{
				NamedVolumes: namedVols,
			}
		}
	}

	return serviceVolumes, nil
}
