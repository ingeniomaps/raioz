package app

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"raioz/internal/config"
	"raioz/internal/domain/interfaces"
	"raioz/internal/host"
)

// getHostServiceInfo gets status information for a host service
func (uc *StatusUseCase) getHostServiceInfo(
	ctx context.Context,
	ws *interfaces.Workspace,
	serviceName string,
	svc config.Service,
	deps *config.Deps,
	hostProcesses map[string]*host.ProcessInfo,
) *interfaces.ServiceInfo {
	info := &interfaces.ServiceInfo{
		Status: "stopped",
		Health: "none",
	}

	// Check if we have ProcessInfo for this service
	processInfo, hasProcessInfo := hostProcesses[serviceName]

	// Get ComposePath: from ProcessInfo first, then from config, then auto-detect
	var composePathToCheck string
	if hasProcessInfo && processInfo.ComposePath != "" {
		composePathToCheck = processInfo.ComposePath
	} else if svc.Commands != nil && svc.Commands.ComposePath != "" {
		// Try to get from config if not in ProcessInfo
		servicePath := uc.deps.Workspace.GetServicePath(ws, serviceName, svc)
		if filepath.IsAbs(svc.Commands.ComposePath) {
			composePathToCheck = svc.Commands.ComposePath
		} else {
			composePathToCheck = filepath.Join(servicePath, svc.Commands.ComposePath)
		}
	} else {
		// Try auto-detect composePath
		servicePath := uc.deps.Workspace.GetServicePath(ws, serviceName, svc)
		command := ""
		if svc.Source.Command != "" {
			command = svc.Source.Command
		} else if svc.Commands != nil && svc.Commands.Up != "" {
			command = svc.Commands.Up
		}
		explicitComposePath := ""
		if svc.Commands != nil {
			explicitComposePath = svc.Commands.ComposePath
		}
		composePathToCheck = uc.deps.HostRunner.DetectComposePath(servicePath, command, explicitComposePath)
	}

	// Priority 1: Check docker-compose if service has ComposePath
	if composePathToCheck != "" {
		// Try to get status from docker-compose
		composeStatus, err := uc.deps.DockerRunner.GetServicesStatusWithContext(ctx, composePathToCheck)
		if err == nil && len(composeStatus) > 0 {
			// Look for any running service in that compose file
			// We can't match by service name since compose file might have different service names
			// So if any service is running, consider it as running
			hasRunning := false
			for _, status := range composeStatus {
				if status == "running" {
					hasRunning = true
					info.Status = "running"
					info.Health = "unknown"
					break
				}
			}
			// If compose file exists but no services running, status remains "stopped"
			if !hasRunning {
				info.Status = "stopped"
			}
		}
	}

	// Priority 2: Check health command if available
	if info.Status == "stopped" || info.Health == "none" {
		mode := "dev"
		if svc.Docker != nil && svc.Docker.Mode != "" {
			mode = svc.Docker.Mode
		}

		healthCommand := ""
		if svc.Commands != nil {
			if mode == "prod" && svc.Commands.Prod != nil && svc.Commands.Prod.Health != "" {
				healthCommand = svc.Commands.Prod.Health
			} else if mode == "dev" && svc.Commands.Dev != nil && svc.Commands.Dev.Health != "" {
				healthCommand = svc.Commands.Dev.Health
			} else if svc.Commands.Health != "" {
				healthCommand = svc.Commands.Health
			}
		}

		if healthCommand != "" {
			servicePath := uc.deps.Workspace.GetServicePath(ws, serviceName, svc)
			// Execute health command
			cmdParts := strings.Fields(healthCommand)
			if len(cmdParts) > 0 {
				var cmd *exec.Cmd
				if len(cmdParts) == 1 {
					cmd = exec.CommandContext(ctx, cmdParts[0])
				} else {
					cmd = exec.CommandContext(ctx, cmdParts[0], cmdParts[1:]...)
				}
				cmd.Dir = servicePath

				if output, err := cmd.CombinedOutput(); err == nil {
					outputStr := strings.TrimSpace(string(output))
					// Parse health output (on/off or JSON with status)
					isHealthy := parseHealthCommandOutput(outputStr)
					if isHealthy {
						info.Status = "running"
						info.Health = "healthy"
					} else {
						info.Status = "stopped"
						info.Health = "unhealthy"
					}
				}
			}
		}
	}

	// Priority 3: Check ProcessInfo PID (for background processes)
	if hasProcessInfo && processInfo.PID > 0 {
		if running, err := uc.deps.HostRunner.IsServiceRunning(processInfo.PID); err == nil && running {
			if info.Status == "stopped" {
				info.Status = "running"
			}
			if info.Health == "none" {
				info.Health = "unknown"
			}
			// Calculate uptime from StartTime
			if !processInfo.StartTime.IsZero() {
				uptime := time.Since(processInfo.StartTime)
				info.Uptime = formatUptimeForStatus(uptime)
			}
		}
	}

	return info
}
