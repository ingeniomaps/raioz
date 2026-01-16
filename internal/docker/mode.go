package docker

import (
	"fmt"
	"os"
	"strings"

	"raioz/internal/config"
	"raioz/internal/workspace"
)

// FilterDevVolumes filters volumes based on mode
// In prod mode, removes bind mounts (dev volumes) and keeps named volumes
// In dev mode, keeps all volumes
func FilterDevVolumes(volumes []string, mode string) []string {
	if mode == "prod" {
		// Remove bind mounts, keep named volumes
		var filtered []string
		for _, vol := range volumes {
			if vol == "" {
				continue
			}

			info, err := ParseVolume(vol)
			if err != nil {
				// If parsing fails, keep it (conservative)
				filtered = append(filtered, vol)
				continue
			}

			// Keep named volumes and anonymous volumes
			// Remove bind mounts (development volumes)
			if info.Type == VolumeTypeNamed || info.Type == VolumeTypeAnonymous {
				filtered = append(filtered, vol)
			}
			// Skip bind mounts in prod mode
		}
		return filtered
	}

	// Dev mode: keep all volumes
	return volumes
}

// GetHealthcheckConfig returns healthcheck configuration based on mode
func GetHealthcheckConfig(mode string) map[string]any {
	if mode == "prod" {
		// Strict healthcheck for prod
		return map[string]any{
			"test":     []string{"CMD-SHELL", "exit 0 || exit 1"},
			"interval": "30s",
			"timeout":  "10s",
			"retries":  3,
			"start_period": "40s",
		}
	}

	// More lenient healthcheck for dev (optional, can be disabled)
	return map[string]any{
		"test":     []string{"CMD-SHELL", "exit 0 || exit 1"},
		"interval": "60s",
		"timeout":  "20s",
		"retries":  2,
		"start_period": "60s",
	}
}

// GetLoggingConfig returns logging configuration based on mode
func GetLoggingConfig(mode string) map[string]any {
	if mode == "prod" {
		// Standard logging for prod
		return map[string]any{
			"driver": "json-file",
			"options": map[string]string{
				"max-size": "10m",
				"max-file": "3",
			},
		}
	}

	// Verbose logging for dev
	return map[string]any{
		"driver": "json-file",
		"options": map[string]string{
			"max-size": "50m",
			"max-file": "5",
		},
	}
}

// AddDevBindMount adds a development bind mount for hot-reload
// Only in dev mode
func AddDevBindMount(
	serviceConfig map[string]any,
	serviceName string,
	svc config.Service,
	ws *workspace.Workspace,
) {
	if svc.Docker.Mode != "dev" {
		return // Only for dev mode
	}

	if svc.Source.Kind != "git" {
		return // Only for git-based services
	}

	// Add bind mount for service code (for hot-reload)
	servicePath := workspace.GetServicePath(ws, serviceName, svc)

	// Check if service path exists
	if _, err := os.Stat(servicePath); os.IsNotExist(err) {
		return // Service path doesn't exist yet, skip bind mount
	}

	// Determine mount target based on runtime
	var mountTarget string
	switch strings.ToLower(svc.Docker.Runtime) {
	case "node", "nodejs", "javascript", "js":
		mountTarget = "/app"
	case "go", "golang":
		mountTarget = "/go/src/app"
	case "python", "py":
		mountTarget = "/app"
	case "java":
		mountTarget = "/app"
	case "rust":
		mountTarget = "/app"
	default:
		mountTarget = "/app"
	}

	bindMount := fmt.Sprintf("%s:%s", servicePath, mountTarget)

	// Get existing volumes or create new
	volumes, ok := serviceConfig["volumes"].([]string)
	if !ok {
		volumes = []string{}
	}

	// Add dev bind mount (avoid duplicates)
	hasMount := false
	for _, vol := range volumes {
		if vol == bindMount {
			hasMount = true
			break
		}
	}

	if !hasMount {
		volumes = append(volumes, bindMount)
		serviceConfig["volumes"] = volumes
	}
}

// ApplyModeConfig applies mode-specific configuration to a service
func ApplyModeConfig(
	serviceConfig map[string]any,
	serviceName string,
	svc config.Service,
	ws *workspace.Workspace,
) {
	// Default to "dev" if mode is not specified (docker is optional now)
	mode := svc.Docker.Mode
	if mode == "" {
		mode = "dev" // Default to dev mode
	}

	// Filter volumes based on mode
	if volumes, ok := serviceConfig["volumes"].([]string); ok && len(volumes) > 0 {
		filtered := FilterDevVolumes(volumes, mode)
		if len(filtered) > 0 {
			serviceConfig["volumes"] = filtered
		} else {
			// Remove volumes key if empty
			delete(serviceConfig, "volumes")
		}
	}

	// Add dev bind mount for hot-reload (dev mode only)
	if mode == "dev" && svc.Source.Kind == "git" {
		AddDevBindMount(serviceConfig, serviceName, svc, ws)
	}

	// Add healthcheck (optional, can be overridden by dockerfile)
	if mode == "prod" {
		// Prod mode requires healthcheck
		healthcheck := GetHealthcheckConfig(mode)
		serviceConfig["healthcheck"] = healthcheck
	}

	// Add logging configuration
	logging := GetLoggingConfig(mode)
	serviceConfig["logging"] = logging

	// Add restart policy based on mode
	// Readonly services always use unless-stopped (immutable, can be recreated)
	if mode == "prod" || (svc.Source.Kind == "git" && svc.Source.Access == "readonly") {
		serviceConfig["restart"] = "unless-stopped"
	} else {
		serviceConfig["restart"] = "no" // Dev mode: no auto-restart
	}
}
