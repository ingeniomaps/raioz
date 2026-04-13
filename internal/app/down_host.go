package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"raioz/internal/config"
	"raioz/internal/domain/interfaces"
	"raioz/internal/host"
	"raioz/internal/i18n"
	"raioz/internal/logging"
	"raioz/internal/output"
)

// stopHostProcesses stops all host processes for a workspace.
// Returns the host processes map (even if empty) so the caller knows if any were found.
func (uc *DownUseCase) stopHostProcesses(
	ctx context.Context,
	ws *interfaces.Workspace,
	opts DownOptions,
) map[string]*host.ProcessInfo {
	hostProcesses, err := uc.deps.HostRunner.LoadProcessesState(ws)
	if err != nil {
		logging.WarnWithContext(ctx, "Failed to load host processes state", "error", err.Error())
		return nil
	}
	if len(hostProcesses) == 0 {
		return hostProcesses
	}

	// Load current config to get stopCommand if not in state
	var currentDeps *config.Deps
	if opts.ConfigPath != "" {
		currentDeps, _, _ = uc.deps.ConfigLoader.LoadDeps(opts.ConfigPath)
	}

	output.PrintInfo(i18n.T("output.stopping_host_services", len(hostProcesses)))
	for name, processInfo := range hostProcesses {
		stopCommand, servicePath := uc.resolveHostStopCommand(ctx, name, *processInfo, currentDeps, ws)

		logging.InfoWithContext(ctx, "Stopping host service",
			"service", name, "pid", processInfo.PID,
			"stopCommand", stopCommand, "servicePath", servicePath)
		err := uc.deps.HostRunner.StopServiceWithCommandAndPath(
			ctx, processInfo.PID, stopCommand, servicePath,
		)
		if err != nil {
			logging.WarnWithContext(ctx, "Failed to stop host service",
				"service", name, "pid", processInfo.PID,
				"error", err.Error())
			output.PrintWarning(i18n.T("output.failed_stop_host", name, processInfo.PID, err))
		} else {
			if stopCommand != "" {
				output.PrintSuccess(i18n.T("output.stopped_host_service_cmd", name, processInfo.PID))
			} else {
				output.PrintSuccess(i18n.T("output.stopped_host_service", name, processInfo.PID))
			}
		}
	}

	// Remove host processes state file
	if err := uc.deps.HostRunner.RemoveProcessesState(ws); err != nil {
		logging.WarnWithContext(ctx, "Failed to remove host processes state", "error", err.Error())
	}

	return hostProcesses
}

// resolveHostStopCommand determines the stop command and service path for a host process.
func (uc *DownUseCase) resolveHostStopCommand(
	ctx context.Context,
	name string,
	processInfo host.ProcessInfo,
	currentDeps *config.Deps,
	ws *interfaces.Workspace,
) (string, string) {
	stopCommand := processInfo.StopCommand
	var servicePath string

	if stopCommand == "" && currentDeps != nil {
		if svc, exists := currentDeps.Services[name]; exists && svc.Commands != nil {
			mode := "dev"
			if svc.Docker != nil && svc.Docker.Mode != "" {
				mode = svc.Docker.Mode
			}

			if mode == "prod" && svc.Commands.Prod != nil && svc.Commands.Prod.Down != "" {
				stopCommand = svc.Commands.Prod.Down
			} else if mode == "dev" && svc.Commands.Dev != nil && svc.Commands.Dev.Down != "" {
				stopCommand = svc.Commands.Dev.Down
			} else if svc.Commands.Down != "" {
				stopCommand = svc.Commands.Down
			}

			if svc.Source.Kind == "git" {
				servicePath = uc.deps.Workspace.GetServicePath(ws, name, svc)
			}

			if stopCommand != "" {
				logging.InfoWithContext(ctx, "Using stopCommand from current config", "service", name, "stopCommand", stopCommand)
			}
		}
	} else if currentDeps != nil {
		if svc, exists := currentDeps.Services[name]; exists && svc.Source.Kind == "git" {
			servicePath = uc.deps.Workspace.GetServicePath(ws, name, svc)
		}
	}

	// If no stopCommand, try to detect docker-compose.yml
	if stopCommand == "" {
		composePathToUse := uc.detectHostComposePath(ctx, name, processInfo, currentDeps, ws, servicePath)
		if composePathToUse != "" {
			composeDir := filepath.Dir(composePathToUse)
			stopCommand = fmt.Sprintf("docker compose -f %s down", composePathToUse)
			if servicePath == "" {
				servicePath = composeDir
			}
			logging.InfoWithContext(ctx,
				"Using docker-compose down for host service",
				"service", name, "composePath", composePathToUse,
				"composeDir", composeDir)
		}
	}

	return stopCommand, servicePath
}

// detectHostComposePath tries to find a docker-compose.yml for a host service.
func (uc *DownUseCase) detectHostComposePath(
	ctx context.Context,
	name string,
	processInfo host.ProcessInfo,
	currentDeps *config.Deps,
	ws *interfaces.Workspace,
	servicePath string,
) string {
	// First, try to use ComposePath from ProcessInfo
	if processInfo.ComposePath != "" {
		if _, err := os.Stat(processInfo.ComposePath); err == nil {
			logging.InfoWithContext(ctx, "Using ComposePath from ProcessInfo",
				"service", name, "composePath", processInfo.ComposePath)
			return processInfo.ComposePath
		}
		logging.WarnWithContext(ctx,
			"ComposePath from ProcessInfo does not exist, trying to detect",
			"service", name, "composePath", processInfo.ComposePath)
	}

	if currentDeps == nil {
		return ""
	}

	svc, exists := currentDeps.Services[name]
	if !exists {
		return ""
	}

	if servicePath == "" && svc.Source.Kind == "git" {
		servicePath = uc.deps.Workspace.GetServicePath(ws, name, svc)
	}

	if servicePath == "" {
		return ""
	}

	var explicitComposePath string
	if svc.Commands != nil {
		explicitComposePath = svc.Commands.ComposePath
	}

	command := ""
	if svc.Source.Command != "" {
		command = svc.Source.Command
	} else if svc.Commands != nil && svc.Commands.Up != "" {
		command = svc.Commands.Up
	}

	detected := uc.deps.HostRunner.DetectComposePath(servicePath, command, explicitComposePath)
	if detected != "" {
		logging.InfoWithContext(ctx,
			"Detected docker-compose.yml for host service",
			"service", name, "composePath", detected,
			"servicePath", servicePath)
	}
	return detected
}
