package app

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"raioz/internal/config"
	"raioz/internal/domain/interfaces"
	"raioz/internal/i18n"
	"raioz/internal/output"
)

// outputJSON outputs status in JSON format
func (uc *StatusUseCase) outputJSON(servicesInfo map[string]*interfaces.ServiceInfo, disabledServices []string, stateDeps *config.Deps, activeWorkspace string) error {
	jsonData := map[string]any{
		"project": map[string]string{
			"name":    stateDeps.Project.Name,
			"network": stateDeps.Network.GetName(),
		},
		"services":      servicesInfo,
		"disabled":      disabledServices,
		"disabledCount": len(disabledServices),
	}
	if activeWorkspace != "" {
		jsonData["activeWorkspace"] = activeWorkspace
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(jsonData); err != nil {
		return fmt.Errorf("failed to encode JSON: %w", err)
	}
	return nil
}

// outputHumanReadable outputs status in human-readable format
func (uc *StatusUseCase) outputHumanReadable(servicesInfo map[string]*interfaces.ServiceInfo, disabledServices []string, stateDeps *config.Deps, activeWorkspace string) error {
	// Table output - these are user-facing output, not logs
	output.PrintSectionHeader(i18n.T("output.project_status_header"))
	output.PrintKeyValue("Project", stateDeps.Project.Name)
	networkName := stateDeps.Network.GetName()
	if stateDeps.Network.HasSubnet() {
		networkName = fmt.Sprintf("%s (%s)", networkName, stateDeps.Network.GetSubnet())
	}
	output.PrintKeyValue("Network", networkName)

	// Show active workspace if set
	if activeWorkspace != "" {
		output.PrintKeyValue(i18n.T("output.active_workspace"), activeWorkspace)
	}

	output.PrintSubsection(i18n.T("output.services_header"))
	if len(servicesInfo) == 0 {
		output.PrintEmptyState("services running")
	} else {
		if err := uc.deps.DockerRunner.FormatStatusTable(servicesInfo, false); err != nil {
			return fmt.Errorf("failed to format table: %w", err)
		}
	}

	// Show disabled services if any
	if len(disabledServices) > 0 {
		output.PrintSubsection(i18n.T("output.disabled_services_header", len(disabledServices)))
		output.PrintList(disabledServices, 0)
	}

	return nil
}

// parseHealthCommandOutput parses health command output (same logic as in upcase)
func parseHealthCommandOutput(output string) bool {
	output = strings.TrimSpace(output)
	outputLower := strings.ToLower(output)

	// Check for "on" or "off"
	if outputLower == "on" {
		return true
	}
	if outputLower == "off" {
		return false
	}

	// Try to parse as JSON
	var jsonData map[string]interface{}
	if err := json.Unmarshal([]byte(output), &jsonData); err == nil {
		if status, ok := jsonData["status"].(string); ok {
			statusLower := strings.ToLower(status)
			if statusLower == "active" || statusLower == "running" || statusLower == "healthy" ||
				statusLower == "up" || statusLower == "on" {
				return true
			}
			if statusLower == "inactive" || statusLower == "stopped" || statusLower == "unhealthy" ||
				statusLower == "down" || statusLower == "off" {
				return false
			}
		}
		return true // JSON without status field defaults to healthy
	}

	// Default: any output with exit code 0 is healthy
	return true
}

// formatUptimeForStatus formats duration for status display
func formatUptimeForStatus(d time.Duration) string {
	days := int(d.Hours() / 24)
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}
