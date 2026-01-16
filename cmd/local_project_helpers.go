package cmd

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"raioz/internal/config"
	"raioz/internal/workspace"
)

// isLocalProject checks if the current directory is a local project (not cloned by raioz)
func isLocalProject(configPath string) (bool, string, error) {
	absConfigPath, err := filepath.Abs(configPath)
	if err != nil {
		return false, "", err
	}
	projectDir := filepath.Dir(absConfigPath)
	baseDir, err := workspace.GetBaseDir()
	if err != nil {
		return false, "", err
	}
	workspacesDir := filepath.Join(baseDir, "workspaces")
	if strings.HasPrefix(projectDir, workspacesDir) {
		return false, "", nil
	}
	servicesDir := filepath.Join(baseDir, "services")
	if strings.HasPrefix(projectDir, servicesDir) {
		return false, "", nil
	}
	return true, projectDir, nil
}

// getLocalProjectCommand gets the command to execute for the local project
func getLocalProjectCommand(deps *config.Deps, commandType string, mode string) string {
	if deps.Project.Commands == nil {
		return ""
	}
	if mode == "" {
		mode = "dev"
	}
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

// executeLocalProjectCommand executes a command for the local project
func executeLocalProjectCommand(ctx context.Context, projectDir string, command string, mode string) error {
	if command == "" {
		return nil
	}
	cmdParts := strings.Fields(command)
	if len(cmdParts) == 0 {
		return nil
	}
	var cmd *exec.Cmd
	if len(cmdParts) == 1 {
		cmd = exec.CommandContext(ctx, cmdParts[0])
	} else {
		cmd = exec.CommandContext(ctx, cmdParts[0], cmdParts[1:]...)
	}
	cmd.Dir = projectDir
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "RAIOZ_MODE="+mode)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// checkLocalProjectHealth checks if the local project is running
func checkLocalProjectHealth(ctx context.Context, projectDir string, healthCommand string) (bool, error) {
	if healthCommand == "" {
		// No health command, return false (not healthy) so up/down can proceed normally
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
	err := cmd.Run()
	if err != nil {
		// Command failed, project is not healthy
		return false, nil
	}
	// Command succeeded, project is healthy
	return true, nil
}
