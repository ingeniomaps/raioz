package production

import (
	"fmt"
	"sort"
	"strings"

	"raioz/internal/config"
)

// CompareConfigs compares local .raioz.json with production docker-compose.yml
func CompareConfigs(local *config.Deps, prod *ProductionConfig) *ComparisonResult {
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

	// Check for services only in production
	for name := range prodServiceMap {
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
func compareService(name string, local *config.Service, prod *ProductionService) *ServiceDifference {
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

	// Compare ports
	localPorts := local.Docker.Ports
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
	localVolumes := local.Docker.Volumes
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
	localDepends := local.Docker.DependsOn
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
func compareInfra(local *config.Deps, prod *ProductionConfig, result *ComparisonResult) {
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
		localPorts := localInf.Ports
		if localPorts == nil {
			localPorts = []string{}
		}
		prodPorts := NormalizePorts(prodSvc.Ports)

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

// FormatComparisonResult formats a comparison result as a readable string
func FormatComparisonResult(result *ComparisonResult) string {
	var sb strings.Builder

	if len(result.Errors) > 0 {
		sb.WriteString("\n❌ Errors:\n")
		for _, err := range result.Errors {
			sb.WriteString(fmt.Sprintf("  • %s\n", err))
		}
	}

	if len(result.ServiceDifferences) > 0 {
		sb.WriteString("\n📊 Service Differences:\n")
		for _, diff := range result.ServiceDifferences {
			sb.WriteString(fmt.Sprintf("\n  Service: %s\n", diff.ServiceName))

			if diff.InLocalOnly {
				sb.WriteString("    ⚠️  Only in local configuration\n")
			}
			if diff.InProductionOnly {
				sb.WriteString("    ℹ️  Only in production configuration\n")
			}

			if diff.ImageMismatch != nil {
				sb.WriteString(fmt.Sprintf("    Image mismatch:\n"))
				sb.WriteString(fmt.Sprintf("      Local:      %s\n", diff.ImageMismatch.Local))
				sb.WriteString(fmt.Sprintf("      Production: %s\n", diff.ImageMismatch.Production))
				if diff.ImageMismatch.LocalTag != diff.ImageMismatch.ProdTag {
					sb.WriteString(fmt.Sprintf("      Tag mismatch: %s vs %s\n",
						diff.ImageMismatch.LocalTag, diff.ImageMismatch.ProdTag))
				}
			}

			if diff.PortMismatch != nil {
				sb.WriteString(fmt.Sprintf("    Port mismatch:\n"))
				sb.WriteString(fmt.Sprintf("      Local:      %v\n", diff.PortMismatch.Local))
				sb.WriteString(fmt.Sprintf("      Production: %v\n", diff.PortMismatch.Production))
			}

			if diff.DependsMismatch != nil {
				sb.WriteString(fmt.Sprintf("    ⚠️  Dependencies mismatch:\n"))
				sb.WriteString(fmt.Sprintf("      Local:      %v\n", diff.DependsMismatch.Local))
				sb.WriteString(fmt.Sprintf("      Production: %v\n", diff.DependsMismatch.Production))
			}

			if diff.VolumeMismatch != nil {
				sb.WriteString(fmt.Sprintf("    Volumes mismatch:\n"))
				sb.WriteString(fmt.Sprintf("      Local:      %v\n", diff.VolumeMismatch.Local))
				sb.WriteString(fmt.Sprintf("      Production: %v\n", diff.VolumeMismatch.Production))
			}
		}
	}

	if len(result.InfraDifferences) > 0 {
		sb.WriteString("\n🏗️  Infrastructure Differences:\n")
		for _, diff := range result.InfraDifferences {
			sb.WriteString(fmt.Sprintf("\n  Infrastructure: %s\n", diff.InfraName))

			if diff.InLocalOnly {
				sb.WriteString("    ⚠️  Only in local configuration\n")
			}
			if diff.InProductionOnly {
				sb.WriteString("    ℹ️  Only in production configuration\n")
			}

			if diff.ImageMismatch != nil {
				sb.WriteString(fmt.Sprintf("    Image mismatch:\n"))
				sb.WriteString(fmt.Sprintf("      Local:      %s\n", diff.ImageMismatch.Local))
				sb.WriteString(fmt.Sprintf("      Production: %s\n", diff.ImageMismatch.Production))
			}

			if diff.PortMismatch != nil {
				sb.WriteString(fmt.Sprintf("    Port mismatch:\n"))
				sb.WriteString(fmt.Sprintf("      Local:      %v\n", diff.PortMismatch.Local))
				sb.WriteString(fmt.Sprintf("      Production: %v\n", diff.PortMismatch.Production))
			}
		}
	}

	if len(result.Warnings) > 0 {
		sb.WriteString("\n⚠️  Warnings:\n")
		for _, warning := range result.Warnings {
			sb.WriteString(fmt.Sprintf("  • %s\n", warning))
		}
	}

	if len(result.ServiceDifferences) == 0 && len(result.InfraDifferences) == 0 &&
		len(result.Errors) == 0 && len(result.Warnings) == 0 {
		sb.WriteString("\n✅ No differences found. Local configuration matches production.\n")
	}

	return sb.String()
}
