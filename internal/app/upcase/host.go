package upcase

import (
	"context"
	"fmt"

	"raioz/internal/config"
	"raioz/internal/domain/interfaces"
	"raioz/internal/host"
	"raioz/internal/i18n"
	"raioz/internal/logging"
	"raioz/internal/output"
)

// saveHostProcessesState saves the host processes state to disk
func (uc *UseCase) saveHostProcessesState(
	ctx context.Context, ws *interfaces.Workspace, processes map[string]*host.ProcessInfo,
) error {
	return uc.deps.HostRunner.SaveProcessesState(ws, processes)
}

// processHostServices starts services that run directly on the host (without Docker)
// projectDir is the directory where .raioz.json is located (used for local services with path: ".")
func (uc *UseCase) processHostServices(
	ctx context.Context, deps *config.Deps, ws *interfaces.Workspace, projectDir string,
) (map[string]*host.ProcessInfo, error) {
	// Collect host services:
	// 1. Services with source.command (host execution)
	// 2. Services with custom commands (no docker, no source.command, but has commands)
	var hostServices []string
	var hostProcessInfo = make(map[string]*host.ProcessInfo)

	for name, svc := range deps.Services {
		// Skip disabled services
		if svc.Enabled != nil && !*svc.Enabled {
			continue
		}

		// Skip Docker services
		if svc.Docker != nil {
			continue
		}

		// Check if source.command exists (host execution)
		if svc.Source.Command != "" {
			hostServices = append(hostServices, name)
		} else if svc.Commands != nil {
			// Service with custom commands (no docker, no source.command)
			hostServices = append(hostServices, name)
		}
	}

	if len(hostServices) == 0 {
		return hostProcessInfo, nil // No host services
	}

	// Start each host service
	output.PrintProgress(i18n.T("up.host.starting_services", len(hostServices)))
	logging.DebugWithContext(ctx, "Starting host services", "count", len(hostServices), "services", hostServices)

	for _, name := range hostServices {
		svc := deps.Services[name]

		// Determine mode
		mode := "dev"
		if svc.Docker != nil && svc.Docker.Mode != "" {
			mode = svc.Docker.Mode
		}

		// Check health before starting (for up command)
		// For services, always check health if available
		healthCommand := getServiceHealthCommand(svc, mode)
		if healthCommand != "" {
			isHealthy, err := checkServiceHealth(ctx, ws, name, svc, mode, uc.deps.Workspace)
			if err != nil {
				logging.WarnWithContext(ctx, "Failed to check service health", "service", name, "error", err.Error())
				// Continue anyway
			} else {
				if isHealthy {
					logging.InfoWithContext(ctx, "Service is already healthy, skipping start", "service", name)
					output.PrintInfo(i18n.T("up.host.already_healthy", name))
					continue
				}
			}
		} else {
			// No health command, use default health check
			isHealthy, err := checkServiceHealth(ctx, ws, name, svc, mode, uc.deps.Workspace)
			if err == nil && isHealthy {
				logging.InfoWithContext(ctx,
					"Service is already healthy (default check), skipping start",
					"service", name)
				output.PrintInfo(i18n.T("up.host.already_healthy", name))
				continue
			}
		}

		// Determine command to use
		// Priority order: docker > source.command > service.commands >
		// service's .raioz.json project.commands > root project.commands
		// IMPORTANT: If source.command exists, it MUST be used (it's the explicit command for host execution)
		var command string
		if svc.Source.Command != "" {
			// Priority 1: Use source.command if available (explicit host execution command)
			command = svc.Source.Command
			logging.DebugWithContext(ctx, "Using source.command for host execution",
				"service", name, "command", command)
		} else if svc.Commands != nil {
			// Priority 2: Use service's commands if available (no source.command)
			// Get command based on mode
			if mode == "prod" && svc.Commands.Prod != nil && svc.Commands.Prod.Up != "" {
				command = svc.Commands.Prod.Up
			} else if mode == "dev" && svc.Commands.Dev != nil && svc.Commands.Dev.Up != "" {
				command = svc.Commands.Dev.Up
			} else if svc.Commands.Up != "" {
				command = svc.Commands.Up
			}
		}

		// Priority 3: If still no command, check if cloned repo has .raioz.json with project.commands
		// Only check if source.command was NOT specified (to avoid overriding explicit commands)
		if command == "" && svc.Source.Kind == "git" {
			servicePath := uc.deps.Workspace.GetServicePath(ws, name, svc)
			svcCfg, _, cfgErr := uc.deps.ConfigLoader.FindServiceConfig(servicePath)
			if cfgErr == nil {
				// Found .raioz.json in cloned repo, check for project.commands
				cmds := svcCfg.Project.Commands
				if cmds != nil {
					logMsg := "Using project.commands.up from service's .raioz.json"
					if mode == "prod" && cmds.Prod != nil && cmds.Prod.Up != "" {
						command = cmds.Prod.Up
						logging.DebugWithContext(ctx, logMsg,
							"service", name, "source", "service_config")
					} else if mode == "dev" && cmds.Dev != nil && cmds.Dev.Up != "" {
						command = cmds.Dev.Up
						logging.DebugWithContext(ctx, logMsg,
							"service", name, "source", "service_config")
					} else if cmds.Up != "" {
						command = cmds.Up
						logging.DebugWithContext(ctx, logMsg,
							"service", name, "source", "service_config")
					}
				}
			}
		}

		// Priority 4: Fallback to root project.commands
		// Only if source.command was NOT specified
		if command == "" {
			if deps.Project.Commands != nil {
				if mode == "prod" && deps.Project.Commands.Prod != nil && deps.Project.Commands.Prod.Up != "" {
					command = deps.Project.Commands.Prod.Up
				} else if mode == "dev" && deps.Project.Commands.Dev != nil && deps.Project.Commands.Dev.Up != "" {
					command = deps.Project.Commands.Dev.Up
				} else if deps.Project.Commands.Up != "" {
					command = deps.Project.Commands.Up
				}
			}
		}

		if command == "" {
			logging.ErrorWithContext(ctx, "Host service missing command", "service", name)
			return nil, fmt.Errorf(
				"service %s requires a command (source.command, commands.up, "+
					"service's .raioz.json project.commands.up, or root project.commands.up)",
				name,
			)
		}

		// Get stop command if available
		var stopCommand string
		if svc.Commands != nil {
			mode := "dev"
			if svc.Docker != nil && svc.Docker.Mode != "" {
				mode = svc.Docker.Mode
			}

			// Get stop command based on mode
			if mode == "prod" && svc.Commands.Prod != nil && svc.Commands.Prod.Down != "" {
				stopCommand = svc.Commands.Prod.Down
			} else if mode == "dev" && svc.Commands.Dev != nil && svc.Commands.Dev.Down != "" {
				stopCommand = svc.Commands.Dev.Down
			} else if svc.Commands.Down != "" {
				stopCommand = svc.Commands.Down
			} else if deps.Project.Commands != nil {
				// Try global command as fallback
				if mode == "prod" && deps.Project.Commands.Prod != nil && deps.Project.Commands.Prod.Down != "" {
					stopCommand = deps.Project.Commands.Prod.Down
				} else if mode == "dev" && deps.Project.Commands.Dev != nil && deps.Project.Commands.Dev.Down != "" {
					stopCommand = deps.Project.Commands.Dev.Down
				} else if deps.Project.Commands.Down != "" {
					stopCommand = deps.Project.Commands.Down
				}
			}
		}

		// Create a temporary service config with the command for StartService
		svcWithCommand := svc
		if svc.Source.Command == "" {
			// Set source.command temporarily so StartService can use it
			svcWithCommand.Source.Command = command
		}

		// Start service
		processInfo, err := uc.deps.HostRunner.StartService(ctx, ws, deps, name, svcWithCommand, projectDir)
		if err != nil {
			logging.DebugWithContext(ctx, "Failed to start host service", "service", name, "error", err.Error())
			output.PrintProgressError(i18n.T("up.host.start_error", name))
			// The error message already includes the command output, so just return it
			return nil, err
		}

		// Store stop command in process info
		if stopCommand != "" {
			processInfo.StopCommand = stopCommand
		}

		hostProcessInfo[name] = processInfo
		output.PrintSuccess(i18n.T("up.host.started", name))
		logging.DebugWithContext(ctx, "Host service started",
			"service", name, "pid", processInfo.PID, "command", processInfo.Command)
	}

	output.PrintProgressDone(i18n.T("up.host.all_started", len(hostServices)))
	return hostProcessInfo, nil
}

// stopHostServices stops running host services
func (uc *UseCase) stopHostServices(ctx context.Context, processInfoMap map[string]*host.ProcessInfo) error {
	if len(processInfoMap) == 0 {
		return nil // No host services to stop
	}

	for name, processInfo := range processInfoMap {
		logging.InfoWithContext(ctx, "Stopping host service",
			"service", name, "pid", processInfo.PID, "stopCommand", processInfo.StopCommand)
		err := uc.deps.HostRunner.StopServiceWithCommand(ctx, processInfo.PID, processInfo.StopCommand)
		if err != nil {
			logging.WarnWithContext(ctx, "Failed to stop host service",
				"service", name, "pid", processInfo.PID, "error", err.Error())
			// Continue stopping other services even if one fails
			continue
		}
		logging.InfoWithContext(ctx, "Host service stopped", "service", name, "pid", processInfo.PID)
	}

	return nil
}
