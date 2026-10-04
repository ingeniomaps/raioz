package production

import (
	"fmt"
	"sort"

	"raioz/internal/domain/models"
)

// CompareConfigs compares the local config with a production docker-compose.yml
func CompareConfigs(local *models.Deps, prod *ProductionConfig) *ComparisonResult {
	result := &ComparisonResult{
		ServiceDifferences: []ServiceDifference{},
		InfraDifferences:   []InfraDifference{},
		Warnings:           []string{},
		Errors:             []string{},
	}

	// Collect all service names from both configs
	localServices := make(map[string]bool)
	for name := range local.Services {
		localServices[name] = true
	}

	prodServices := prod.GetServiceNames()
	prodServiceMap := make(map[string]bool)
	for _, name := range prodServices {
		prodServiceMap[name] = true
	}

	// Compare services present in both configs
	for name := range localServices {
		localSvc := local.Services[name]
		prodSvc, inProd := prod.Services[name]

		if !inProd {
			result.ServiceDifferences = append(result.ServiceDifferences, ServiceDifference{
				ServiceName: name,
				InLocalOnly: true,
				Severity:    "warning",
			})
			continue
		}

		diff := compareService(name, &localSvc, &prodSvc)
		if diff != nil {
			result.ServiceDifferences = append(result.ServiceDifferences, *diff)
		}
	}

	// Check for services only in production. A name the local config
	// declares under `dependencies:` is not missing — compareInfra covers
	// it — and one that looks like infrastructure is reported there too.
	for name := range prodServiceMap {
		if _, isLocalDep := local.Infra[name]; isLocalDep || isInfraService(name) {
			continue
		}
		if !localServices[name] {
			result.ServiceDifferences = append(result.ServiceDifferences, ServiceDifference{
				ServiceName:      name,
				InProductionOnly: true,
				Severity:         "info",
			})
		}
	}

	// Compare infra services
	compareInfra(local, prod, result)

	// Sort differences by service name for consistent output
	sort.Slice(result.ServiceDifferences, func(i, j int) bool {
		return result.ServiceDifferences[i].ServiceName < result.ServiceDifferences[j].ServiceName
	})

	sort.Slice(result.InfraDifferences, func(i, j int) bool {
		return result.InfraDifferences[i].InfraName < result.InfraDifferences[j].InfraName
	})

	return result
}

// compareService compares a single service between local and production
func compareService(name string, local *models.Service, prod *ProductionService) *ServiceDifference {
	diff := &ServiceDifference{
		ServiceName: name,
		Severity:    "info",
	}

	// Compare image
	if local.Source.Kind == "image" {
		localImage := fmt.Sprintf("%s:%s", local.Source.Image, local.Source.Tag)
		prodImage, prodTag := ExtractImageAndTag(prod.Image)

		if local.Source.Image != prodImage || local.Source.Tag != prodTag {
			diff.ImageMismatch = &ImageMismatch{
				Local:      localImage,
				Production: prod.Image,
				LocalTag:   local.Source.Tag,
				ProdTag:    prodTag,
			}
			if local.Source.Tag != prodTag {
				diff.Severity = "warning"
			}
		}
	}

	// A service without `ports:` has no docker block at all — the common
	// shape for `command:` services. Treat it as an empty one instead of
	// dereferencing nil.
	localDocker := local.Docker
	if localDocker == nil {
		localDocker = &models.DockerConfig{}
	}

	// Compare ports
	localPorts := localDocker.Ports
	if localPorts == nil {
		localPorts = []string{}
	}
	prodPorts := NormalizePorts(prod.Ports)

	if !portsEqual(localPorts, prodPorts) {
		diff.PortMismatch = &PortMismatch{
			Local:      localPorts,
			Production: prodPorts,
		}
		diff.Severity = "warning"
	}

	// Compare volumes
	localVolumes := localDocker.Volumes
	if localVolumes == nil {
		localVolumes = []string{}
	}
	prodVolumes := prod.Volumes
	if prodVolumes == nil {
		prodVolumes = []string{}
	}

	if !volumesEqual(localVolumes, prodVolumes) {
		diff.VolumeMismatch = &VolumeMismatch{
			Local:      localVolumes,
			Production: prodVolumes,
		}
		diff.Severity = "info" // Volumes often differ between dev/prod
	}

	// Compare dependencies
	// GetDependsOn covers `dependsOn:` from raioz.yaml as well as the
	// legacy docker block; reading only the latter reported every yaml
	// service as depending on nothing.
	localDepends := local.GetDependsOn()
	if localDepends == nil {
		localDepends = []string{}
	}
	prodDepends := ParseDependsOn(prod.DependsOn)

	if !dependsEqual(localDepends, prodDepends) {
		diff.DependsMismatch = &DependsMismatch{
			Local:      localDepends,
			Production: prodDepends,
		}
		diff.Severity = "error" // Dependencies mismatches are critical
	}

	// Only return diff if there are actual differences
	if diff.ImageMismatch == nil && diff.PortMismatch == nil &&
		diff.VolumeMismatch == nil && diff.DependsMismatch == nil {
		return nil
	}

	return diff
}

// compareInfra compares infrastructure services
func compareInfra(local *models.Deps, prod *ProductionConfig, result *ComparisonResult) {
	localInfra := make(map[string]bool)
	for name := range local.Infra {
		localInfra[name] = true
	}

	prodServiceNames := prod.GetServiceNames()
	prodServiceMap := make(map[string]bool)
	for _, name := range prodServiceNames {
		prodServiceMap[name] = true
	}

	// Compare inline infra services (path-based entries are skipped)
	for name := range localInfra {
		localEntry := local.Infra[name]
		if localEntry.Inline == nil {
			continue
		}
		localInf := *localEntry.Inline
		prodSvc, inProd := prod.Services[name]

		if !inProd {
			result.InfraDifferences = append(result.InfraDifferences, InfraDifference{
				InfraName:   name,
				InLocalOnly: true,
				Severity:    "warning",
			})
			continue
		}

		diff := InfraDifference{
			InfraName: name,
			Severity:  "info",
		}

		// Compare image
		localImage := fmt.Sprintf("%s:%s", localInf.Image, localInf.Tag)
		prodImage, prodTag := ExtractImageAndTag(prodSvc.Image)

		if localInf.Image != prodImage || localInf.Tag != prodTag {
			diff.ImageMismatch = &ImageMismatch{
				Local:      localImage,
				Production: prodSvc.Image,
				LocalTag:   localInf.Tag,
				ProdTag:    prodTag,
			}
			if localInf.Tag != prodTag {
				diff.Severity = "warning"
			}
		}

		// Compare ports
		prodPorts := NormalizePorts(prodSvc.Ports)
		localPorts := localInfraPorts(localInf, prodPorts)

		if !portsEqual(localPorts, prodPorts) {
			diff.PortMismatch = &PortMismatch{
				Local:      localPorts,
				Production: prodPorts,
			}
			diff.Severity = "warning"
		}

		// Only add if there are differences
		if diff.ImageMismatch != nil || diff.PortMismatch != nil {
			result.InfraDifferences = append(result.InfraDifferences, diff)
		}
	}

	// Check for infra only in production (less common)
	for name := range prodServiceMap {
		if _, isLocalService := local.Services[name]; isLocalService {
			continue
		}
		if !localInfra[name] {
			// Only mark as infra if it looks like infrastructure (DB, cache, etc.)
			if isInfraService(name) {
				result.InfraDifferences = append(result.InfraDifferences, InfraDifference{
					InfraName:        name,
					InProductionOnly: true,
					Severity:         "info",
				})
			}
		}
	}
}
