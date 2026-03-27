package upcase

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"raioz/internal/config"
	"raioz/internal/i18n"
	"raioz/internal/logging"
	"raioz/internal/output"
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
	baseDir, err := getBaseDirForLocalCheck()
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

// getBaseDirForLocalCheck gets the base directory for local project checking.
// This uses the RAIOZ_HOME env var or default ~/.raioz path directly,
// since this is a utility function that doesn't need the full workspace manager.
func getBaseDirForLocalCheck() (string, error) {
	if home := os.Getenv("RAIOZ_HOME"); home != "" {
		return home, nil
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".raioz"), nil
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
	cmdOutput, err := cmd.CombinedOutput()
	outputStr := strings.TrimSpace(string(cmdOutput))

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

	output.PrintProgress(i18n.T("up.local.executing_command_detail", command))

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

	output.PrintSuccess(i18n.T("up.local.command_success"))
	return nil
}
