package upcase

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"raioz/internal/config"
	"raioz/internal/domain/interfaces"
	"raioz/internal/errors"
	"raioz/internal/logging"
	"raioz/internal/output"
	"raioz/internal/state"
)

// processLocalProject processes local project commands (if this is a local project or has project.commands)
func (uc *UseCase) processLocalProject(ctx context.Context, configPath string, deps *config.Deps, commandType string, ws interface{}) error {
	// Check if this is a local project
	isLocal, projectDir, err := isLocalProject(configPath)
	if err != nil {
		return err
	}

	// If not local but has project commands, use current directory as project dir
	if !isLocal {
		// Check if project has commands defined
		hasProjectCommands := deps.Project.Commands != nil && ((commandType == "up" && (deps.Project.Commands.Up != "" ||
			(deps.Project.Commands.Dev != nil && deps.Project.Commands.Dev.Up != "") ||
			(deps.Project.Commands.Prod != nil && deps.Project.Commands.Prod.Up != ""))) ||
			(commandType == "down" && (deps.Project.Commands.Down != "" ||
				(deps.Project.Commands.Dev != nil && deps.Project.Commands.Dev.Down != "") ||
				(deps.Project.Commands.Prod != nil && deps.Project.Commands.Prod.Down != ""))) ||
			(commandType == "health" && (deps.Project.Commands.Health != "" ||
				(deps.Project.Commands.Dev != nil && deps.Project.Commands.Dev.Health != "") ||
				(deps.Project.Commands.Prod != nil && deps.Project.Commands.Prod.Health != ""))))

		if !hasProjectCommands {
			// Not a local project and no commands, nothing to do
			return nil
		}

		// Has project commands but not local, use current directory
		absConfigPath, err := filepath.Abs(configPath)
		if err != nil {
			return fmt.Errorf("failed to get absolute path: %w", err)
		}
		projectDir = filepath.Dir(absConfigPath)
	}

	// For 'up' command, check and handle duplicate project (running from workspace)
	// Only check if this is a local project with ONLY commands (no services/infra)
	// If it has services/infra, it's already running from workspace and there's no duplicate
	if commandType == "up" {
		// Check if this project has services or infra
		hasServices := len(deps.Services) > 0
		hasInfra := len(deps.Infra) > 0

		// Only check for duplicate if it's a command-only project (no services/infra)
		// If it has services/infra, it's already being managed by the workspace flow
		if !hasServices && !hasInfra {
			// First check for service conflicts (if project name matches a service)
			// This handles the case where a service is running from workspace and we want to run the local project
			serviceConflictResolved := false
			if wsTyped, ok := ws.(*interfaces.Workspace); ok {
				conflict, err := uc.detectServiceConflict(ctx, deps.Project.Name, deps, wsTyped, projectDir, true)
				if err == nil && conflict != nil {
					// Service conflict detected - resolve it (only stop the conflicting service, not infra or other services)
					resolution, err := uc.resolveServiceConflict(ctx, conflict, true, deps.GetWorkspaceName(), projectDir)
					if err != nil {
						return err
					}
					if resolution == "cancel" {
						return errors.New(errors.ErrCodeWorkspaceError, "Operation cancelled by user")
					}
					if resolution == "skip" {
						output.PrintInfo("Keeping current service, skipping local project execution")
						return nil
					}
					// Apply resolution (stops only the conflicting service)
					if err := uc.applyServiceConflictResolution(ctx, conflict, resolution, deps.Project.Name, deps, wsTyped, projectDir, true); err != nil {
						return err
					}
					serviceConflictResolved = true
				}
			}

			// Only check for duplicate project if we did NOT just resolve a service conflict.
			// When we resolved a conflict we only stopped that one service; duplicate check would do a full workspace down (infra + all services).
			if !serviceConflictResolved {
				if err := uc.checkAndHandleDuplicateProject(ctx, deps.Project.Name, configPath); err != nil {
					// Check if it's a user cancellation
					if strings.Contains(err.Error(), "user cancelled") {
						// User cancelled, return error to stop execution
						return errors.New(
							errors.ErrCodeWorkspaceError,
							"Operation cancelled by user",
						).WithSuggestion(
							"The workspace project remains running. To start the local project, first stop the workspace project with 'raioz down'.",
						)
					}
					logging.WarnWithContext(ctx, "Failed to check/handle duplicate project", "error", err.Error())
					// Continue anyway - might be a false positive
				}
			}
		}
	}

	// Determine mode (default to dev)
	mode := "dev"
	// Try to get mode from first service with docker config
	for _, svc := range deps.Services {
		if svc.Docker != nil && svc.Docker.Mode != "" {
			mode = svc.Docker.Mode
			break
		}
	}

	// For up/down, check health first if available
	if commandType == "up" || commandType == "down" {
		healthCommand := getLocalProjectCommand(deps, "health", mode)
		if healthCommand != "" {
			// Check health
			isHealthy, err := checkLocalProjectHealth(ctx, projectDir, healthCommand)
			if err != nil {
				logging.WarnWithContext(ctx, "Failed to check project health", "error", err.Error())
				// Continue anyway
			} else {
				if commandType == "up" {
					if isHealthy {
						logging.InfoWithContext(ctx, "Project is already healthy, skipping up command")
						output.PrintInfo("Project is already running and healthy")
						return nil
					}
				} else if commandType == "down" {
					if !isHealthy {
						// Project is not healthy (not running), nothing to stop
						logging.InfoWithContext(ctx, "Project is not healthy, skipping down command")
						output.PrintInfo("Project is not running, nothing to stop")
						return nil
					}
					// Project is healthy (running), proceed with down
				}
			}
		}
		// If no health command, proceed normally (for project local without health)
	}

	// Generate .env from template for local project (before executing command)
	// Only if ws is provided (not nil)
	if ws != nil {
		// Create a dummy service config for template generation
		var dummyEnv *config.EnvValue
		if deps.Env.Files != nil && len(deps.Env.Files) > 0 {
			dummyEnv = &config.EnvValue{
				Files:     deps.Env.Files,
				Variables: nil,
				IsObject:  false,
			}
		}
		dummyService := config.Service{
			Env: dummyEnv,
		}
		// Type assert to *interfaces.Workspace
		if wsTyped, ok := ws.(*interfaces.Workspace); ok {
			// Resolve project.env for local project
			projectEnvPath, _ := uc.deps.EnvManager.ResolveProjectEnv(wsTyped, deps, projectDir)
			if err := uc.deps.EnvManager.GenerateEnvFromTemplate(wsTyped, deps, deps.Project.Name, projectDir, dummyService, projectEnvPath, projectDir); err != nil {
				logging.WarnWithContext(ctx, "Failed to generate .env from template for local project", "error", err.Error())
				// Continue anyway - template generation is optional
			}
		}
	}

	// Get command to execute
	command := getLocalProjectCommand(deps, commandType, mode)
	if command == "" {
		// No command defined, nothing to do
		return nil
	}

	// Execute command (log at debug level - technical details)
	logging.DebugWithContext(ctx, "Executing local project command",
		"project_dir", projectDir,
		"command_type", commandType,
		"mode", mode,
		"command", command,
	)

	output.PrintInfo("Executing local project command...")
	if err := executeLocalProjectCommand(ctx, projectDir, command, mode); err != nil {
		return err
	}

	// If command was "up" and executed successfully, detect and save project docker-compose.yml
	if commandType == "up" {
		// Detect project docker-compose.yml
		projectComposePath := uc.deps.HostRunner.DetectComposePath(projectDir, command, "")
		if projectComposePath != "" {
			// Save project compose path to state
			deps.ProjectComposePath = projectComposePath
			// Save updated state
			if ws != nil {
				if wsTyped, ok := ws.(*interfaces.Workspace); ok {
					if err := uc.deps.StateManager.Save(wsTyped, deps); err != nil {
						logging.WarnWithContext(ctx, "Failed to save project compose path to state", "error", err.Error())
					} else {
						logging.DebugWithContext(ctx, "Project docker-compose.yml detected and saved",
							"compose_path", projectComposePath,
						)
					}
				}
			}
		}
		if err := uc.saveProjectCommandState(ctx, deps, projectDir); err != nil {
			// Log but don't fail - state saving is optional
			logging.WarnWithContext(ctx, "Failed to save project command state", "error", err.Error())
		}
	}

	return nil
}

// saveProjectCommandState saves the project state to global state when project.commands.up is executed
func (uc *UseCase) saveProjectCommandState(ctx context.Context, deps *config.Deps, projectDir string) error {
	// Create project state for command-based project
	projectState := &state.ProjectState{
		Name:          deps.Project.Name,
		Workspace:     projectDir,
		LastExecution: time.Now(),
		Services:      []state.ServiceState{}, // Empty services for command-based projects
	}

	// Update global state
	if err := uc.deps.StateManager.UpdateProjectState(deps.Project.Name, projectState); err != nil {
		return fmt.Errorf("failed to update global state: %w", err)
	}

	// Only log at debug level - technical detail not useful for end users
	logging.DebugWithContext(ctx, "Project command state saved to global state",
		"project", deps.Project.Name,
		"workspace", projectDir,
	)

	return nil
}
