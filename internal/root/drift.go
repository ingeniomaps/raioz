package root

import (
	"fmt"
	"os"
	"path/filepath"

	"raioz/internal/config"
	"raioz/internal/state"
	"raioz/internal/workspace"
)

// ServiceDrift represents detected differences between service .raioz.json and root config
type ServiceDrift struct {
	ServiceName string
	Differences []state.ConfigChange
	ServicePath string // Path to service directory
}

// DetectAssistedServiceDrift detects changes in services that were added via dependency assist
// Compares .raioz.json of service with configuration in raioz.root.json
// Returns list of drifts detected (only for services with OriginAssisted)
func DetectAssistedServiceDrift(rootConfig *RootConfig, ws *workspace.Workspace) ([]ServiceDrift, error) {
	if rootConfig == nil {
		return nil, nil
	}

	var drifts []ServiceDrift

	// Iterate through all services in root config
	for serviceName, svc := range rootConfig.Services {
		// Check if service was added via dependency assist
		meta, exists := rootConfig.Metadata[serviceName]
		if !exists || meta.Origin != OriginAssisted {
			continue // Skip services not added via assist
		}

		// Get service path - rootConfig.Services is already config.Service
		servicePath := workspace.GetServicePath(ws, serviceName, svc)

		// Look for .raioz.json in service directory
		serviceConfigPath := filepath.Join(servicePath, ".raioz.json")
		if _, err := os.Stat(serviceConfigPath); os.IsNotExist(err) {
			continue // Service doesn't have .raioz.json, skip
		}

		// Load service .raioz.json
		serviceDeps, _, err := config.LoadDeps(serviceConfigPath)
		if err != nil {
			// Failed to load service config, skip (non-fatal)
			continue
		}

		// Find service in service deps
		serviceConfig, exists := serviceDeps.Services[serviceName]
		if !exists {
			continue // Service not in its own .raioz.json, skip
		}

		// Compare service config with root config
		rootServiceConfig := rootConfig.Services[serviceName]

		// Create temporary deps for comparison
		rootDeps := &config.Deps{
			Services: map[string]config.Service{
				serviceName: rootServiceConfig,
			},
		}
		serviceDepsForCompare := &config.Deps{
			Services: map[string]config.Service{
				serviceName: serviceConfig,
			},
		}

		// Use CompareDeps to find differences
		changes, err := state.CompareDeps(rootDeps, serviceDepsForCompare)
		if err != nil {
			// Comparison failed, skip (non-fatal)
			continue
		}

		// Filter out changes where old and new values are the same (shouldn't happen, but safety)
		var realChanges []state.ConfigChange
		for _, change := range changes {
			if change.OldValue != change.NewValue {
				realChanges = append(realChanges, change)
			}
		}

		// If there are differences, add to drifts
		if len(realChanges) > 0 {
			drifts = append(drifts, ServiceDrift{
				ServiceName: serviceName,
				Differences: realChanges,
				ServicePath: serviceConfigPath,
			})
		}
	}

	return drifts, nil
}

// FormatDrift formats a ServiceDrift for display
func FormatDrift(drift ServiceDrift) string {
	var result string
	result += fmt.Sprintf("  Service: %s\n", drift.ServiceName)
	result += fmt.Sprintf("  Config file: %s\n", drift.ServicePath)
	result += fmt.Sprintf("  Differences (%d):\n", len(drift.Differences))
	for i, change := range drift.Differences {
		// Format field name more clearly
		fieldDisplay := change.Field
		if change.Type == "service" {
			fieldDisplay = fmt.Sprintf("service.%s", change.Field)
		}

		// Format old and new values more clearly
		oldVal := change.OldValue
		if oldVal == "" {
			oldVal = "(empty)"
		}
		newVal := change.NewValue
		if newVal == "" {
			newVal = "(empty)"
		}

		result += fmt.Sprintf("    %d. %s\n", i+1, fieldDisplay)
		result += fmt.Sprintf("       Previous: %s\n", oldVal)
		result += fmt.Sprintf("       Current:  %s\n", newVal)
		if i < len(drift.Differences)-1 {
			result += "\n"
		}
	}
	return result
}

// FormatDrifts formats a list of ServiceDrift for display
func FormatDrifts(drifts []ServiceDrift) string {
	if len(drifts) == 0 {
		return ""
	}

	var result string
	result += fmt.Sprintf("\n⚠️  Configuration Drift Detected\n")
	result += fmt.Sprintf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	result += fmt.Sprintf("Detected changes in %d service(s) added via dependency assist:\n\n", len(drifts))

	for i, drift := range drifts {
		result += fmt.Sprintf("┌─ Service: %s\n", drift.ServiceName)
		result += fmt.Sprintf("│  Config: %s\n", drift.ServicePath)
		result += fmt.Sprintf("│  Changes: %d difference(s)\n", len(drift.Differences))
		result += fmt.Sprintf("│\n")

		for j, change := range drift.Differences {
			fieldDisplay := change.Field
			if change.Type == "service" {
				fieldDisplay = fmt.Sprintf("service.%s", change.Field)
			}

			oldVal := change.OldValue
			if oldVal == "" {
				oldVal = "(empty)"
			}
			newVal := change.NewValue
			if newVal == "" {
				newVal = "(empty)"
			}

			prefix := "│"
			if j == len(drift.Differences)-1 {
				prefix = "└"
			}

			result += fmt.Sprintf("%s  %d. %s\n", prefix, j+1, fieldDisplay)
			result += fmt.Sprintf("%s     Previous: %s\n", prefix, oldVal)
			result += fmt.Sprintf("%s     Current:  %s\n", prefix, newVal)
			if j < len(drift.Differences)-1 {
				result += fmt.Sprintf("%s\n", prefix)
			}
		}

		if i < len(drifts)-1 {
			result += "\n"
		}
	}

	result += "\n"
	result += "ℹ️  Note: These differences are informational only.\n"
	result += "   raioz.root.json was not modified automatically.\n"
	result += "   Update raioz.root.json manually if you want to use the service's .raioz.json configuration.\n"
	return result
}
