package upcase

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"raioz/internal/config"
	"raioz/internal/env"
	"raioz/internal/errors"
	"raioz/internal/host"
	"raioz/internal/logging"
	"raioz/internal/output"
	"raioz/internal/state"
	workspacepkg "raioz/internal/workspace"
)

// isLocalProject checks if the current directory is a local project (not cloned by raioz)
func isLocalProject(configPath string) (bool, string, error) {
	// Get absolute path of config file
	absConfigPath, err := filepath.Abs(configPath)
	if err != nil {
		return false, "", fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Get the directory containing the config file
	projectDir := filepath.Dir(absConfigPath)

	// Check if this directory is inside a raioz workspace
	baseDir, err := workspacepkg.GetBaseDir()
	if err != nil {
		return false, "", fmt.Errorf("failed to get base directory: %w", err)
	}

	// Check if projectDir is inside any workspace
	workspacesDir := filepath.Join(baseDir, "workspaces")
	if strings.HasPrefix(projectDir, workspacesDir) {
		// This is inside a workspace, so it's a cloned project
		return false, "", nil
	}

	// Check if projectDir is inside services directory
	servicesDir := filepath.Join(baseDir, "services")
	if strings.HasPrefix(projectDir, servicesDir) {
		// This is inside services, so it's a cloned service
		return false, "", nil
	}

	// This appears to be a local project
	return true, projectDir, nil
}

// getLocalProjectCommand gets the command to execute for the local project
func getLocalProjectCommand(deps *config.Deps, commandType string, mode string) string {
	if deps.Project.Commands == nil {
		return ""
	}

	// Determine mode (default to dev)
	if mode == "" {
		mode = "dev"
	}

	// Get command based on type (up/down/health) and mode
	if commandType == "up" {
		if mode == "prod" && deps.Project.Commands.Prod != nil && deps.Project.Commands.Prod.Up != "" {
			return deps.Project.Commands.Prod.Up
		}
		if mode == "dev" && deps.Project.Commands.Dev != nil && deps.Project.Commands.Dev.Up != "" {
			return deps.Project.Commands.Dev.Up
		}
		if deps.Project.Commands.Up != "" {
			return deps.Project.Commands.Up
		}
	} else if commandType == "down" {
		if mode == "prod" && deps.Project.Commands.Prod != nil && deps.Project.Commands.Prod.Down != "" {
			return deps.Project.Commands.Prod.Down
		}
		if mode == "dev" && deps.Project.Commands.Dev != nil && deps.Project.Commands.Dev.Down != "" {
			return deps.Project.Commands.Dev.Down
		}
		if deps.Project.Commands.Down != "" {
			return deps.Project.Commands.Down
		}
	} else if commandType == "health" {
		if mode == "prod" && deps.Project.Commands.Prod != nil && deps.Project.Commands.Prod.Health != "" {
			return deps.Project.Commands.Prod.Health
		}
		if mode == "dev" && deps.Project.Commands.Dev != nil && deps.Project.Commands.Dev.Health != "" {
			return deps.Project.Commands.Dev.Health
		}
		if deps.Project.Commands.Health != "" {
			return deps.Project.Commands.Health
		}
	}

	return ""
}

// checkLocalProjectHealth checks if the local project is running
func checkLocalProjectHealth(ctx context.Context, projectDir string, healthCommand string) (bool, error) {
	if healthCommand == "" {
		// No health command defined, return false (not healthy) so up/down can proceed normally
		return false, nil
	}

	cmdParts := strings.Fields(healthCommand)
	if len(cmdParts) == 0 {
		return false, nil
	}

	var cmd *exec.Cmd
	if len(cmdParts) == 1 {
		cmd = exec.CommandContext(ctx, cmdParts[0])
	} else {
		cmd = exec.CommandContext(ctx, cmdParts[0], cmdParts[1:]...)
	}

	cmd.Dir = projectDir
	cmd.Env = os.Environ()

	// Capture stdout to parse response
	output, err := cmd.CombinedOutput()
	outputStr := strings.TrimSpace(string(output))

	if err != nil {
		// Command failed, project is not healthy
		return false, nil
	}

	// Parse output to determine health status (same logic as service health)
	return parseHealthCommandOutput(outputStr), nil
}

// executeLocalProjectCommand executes a command for the local project
func executeLocalProjectCommand(ctx context.Context, projectDir string, command string, mode string) error {
	if command == "" {
		return nil // No command to execute
	}

	// Log at debug level - technical detail
	logging.DebugWithContext(ctx, "Executing local project command",
		"project_dir", projectDir,
		"command", command,
		"mode", mode,
	)

	output.PrintProgress(fmt.Sprintf("Executing local project command: %s", command))

	// Parse command (simple split for now)
	cmdParts := strings.Fields(command)
	if len(cmdParts) == 0 {
		return fmt.Errorf("empty command")
	}

	// Create command
	var cmd *exec.Cmd
	if len(cmdParts) == 1 {
		cmd = exec.CommandContext(ctx, cmdParts[0])
	} else {
		cmd = exec.CommandContext(ctx, cmdParts[0], cmdParts[1:]...)
	}

	// Set working directory to project directory
	cmd.Dir = projectDir

	// Set environment variables (inherit from current process)
	cmd.Env = os.Environ()

	// Add RAIOZ_MODE environment variable
	cmd.Env = append(cmd.Env, fmt.Sprintf("RAIOZ_MODE=%s", mode))

	// Execute command
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to execute local project command: %w", err)
	}

	output.PrintSuccess("Local project command executed successfully")
	return nil
}

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
		hasProjectCommands := deps.Project.Commands != nil && (
			(commandType == "up" && (deps.Project.Commands.Up != "" ||
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
		// Type assert to *workspace.Workspace
		if wsConcrete, ok := ws.(*workspacepkg.Workspace); ok {
			// Resolve project.env for local project
			projectEnvPath, _ := env.ResolveProjectEnv(wsConcrete, deps, projectDir)
			if err := env.GenerateEnvFromTemplate(wsConcrete, deps, deps.Project.Name, projectDir, dummyService, projectEnvPath); err != nil {
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
		projectComposePath := host.DetectComposePath(projectDir, command, "")
		if projectComposePath != "" {
			// Save project compose path to state
			deps.ProjectComposePath = projectComposePath
			// Save updated state
			if ws != nil {
				if wsConcrete, ok := ws.(*workspacepkg.Workspace); ok {
					if err := state.Save(wsConcrete, deps); err != nil {
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
	projectState := state.ProjectState{
		Name:          deps.Project.Name,
		Workspace:     projectDir,
		LastExecution: time.Now(),
		Services:      []state.ServiceState{}, // Empty services for command-based projects
	}

	// Update global state
	if err := state.UpdateProjectState(deps.Project.Name, projectState); err != nil {
		return fmt.Errorf("failed to update global state: %w", err)
	}

	// Only log at debug level - technical detail not useful for end users
	logging.DebugWithContext(ctx, "Project command state saved to global state",
		"project", deps.Project.Name,
		"workspace", projectDir,
	)

	return nil
}
