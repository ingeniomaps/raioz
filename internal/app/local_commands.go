package app

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"raioz/internal/config"
	"raioz/internal/errors"
	"raioz/internal/i18n"
	"raioz/internal/logging"
	"raioz/internal/output"
)

// IsLocalProject checks if the config path points to a local project (not inside raioz workspace)
func IsLocalProject(configPath string, baseDir string) (bool, string, error) {
	absConfigPath, err := filepath.Abs(configPath)
	if err != nil {
		return false, "", fmt.Errorf("abs %q: %w", configPath, err)
	}
	projectDir := filepath.Dir(absConfigPath)
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

// GetLocalProjectCommand gets the command to execute for a local project based on type and mode
func GetLocalProjectCommand(deps *config.Deps, commandType string, mode string) string {
	if deps.Project.Commands == nil {
		return ""
	}
	if mode == "" {
		mode = "dev"
	}

	switch commandType {
	case "up":
		if mode == "prod" && deps.Project.Commands.Prod != nil && deps.Project.Commands.Prod.Up != "" {
			return deps.Project.Commands.Prod.Up
		}
		if mode == "dev" && deps.Project.Commands.Dev != nil && deps.Project.Commands.Dev.Up != "" {
			return deps.Project.Commands.Dev.Up
		}
		return deps.Project.Commands.Up
	case "down":
		if mode == "prod" && deps.Project.Commands.Prod != nil && deps.Project.Commands.Prod.Down != "" {
			return deps.Project.Commands.Prod.Down
		}
		if mode == "dev" && deps.Project.Commands.Dev != nil && deps.Project.Commands.Dev.Down != "" {
			return deps.Project.Commands.Dev.Down
		}
		return deps.Project.Commands.Down
	case "health":
		if mode == "prod" && deps.Project.Commands.Prod != nil && deps.Project.Commands.Prod.Health != "" {
			return deps.Project.Commands.Prod.Health
		}
		if mode == "dev" && deps.Project.Commands.Dev != nil && deps.Project.Commands.Dev.Health != "" {
			return deps.Project.Commands.Dev.Health
		}
		return deps.Project.Commands.Health
	}
	return ""
}

// ExecuteLocalProjectCommand executes a shell command in the project directory
func ExecuteLocalProjectCommand(ctx context.Context, projectDir string, command string, mode string) error {
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
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("run local project command: %w", err)
	}
	return nil
}

// CheckLocalProjectHealth checks if a local project is running via its health command
func CheckLocalProjectHealth(ctx context.Context, projectDir string, healthCommand string) (bool, error) {
	if healthCommand == "" {
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
	return cmd.Run() == nil, nil
}

// HandleLocalProjectDown handles the local project down command logic.
// Returns (handled bool, err error) — if handled is true, the caller should return err directly.
func HandleLocalProjectDown(ctx context.Context, configPath string, baseDir string, downErr error) (bool, error) {
	configDeps, _, loadErr := config.LoadDeps(configPath)
	if loadErr != nil || configDeps == nil {
		return false, downErr
	}

	isLocal, projectDir, checkErr := IsLocalProject(configPath, baseDir)
	if checkErr != nil || !isLocal {
		return false, downErr
	}

	// Determine mode
	mode := "dev"
	for _, svc := range configDeps.Services {
		if svc.Docker != nil && svc.Docker.Mode != "" {
			mode = svc.Docker.Mode
			break
		}
	}

	downCommand := GetLocalProjectCommand(configDeps, "down", mode)
	if downCommand == "" {
		return false, downErr
	}

	// Check health before down
	healthCommand := GetLocalProjectCommand(configDeps, "health", mode)
	if healthCommand != "" {
		isHealthy, healthErr := CheckLocalProjectHealth(ctx, projectDir, healthCommand)
		if healthErr == nil && !isHealthy {
			logging.InfoWithContext(ctx, "Project is not healthy, skipping down command")
			output.PrintInfo(i18n.T("output.project_not_running"))
			return true, nil
		}
	}

	if downErr != nil {
		if raiozErr, ok := downErr.(*errors.RaiozError); ok && raiozErr.Code == errors.ErrCodeStateLoadError {
			output.PrintInfo(i18n.T("output.no_raioz_state_exec_down"))
		} else {
			output.PrintInfo(i18n.T("output.executing_down_cmd"))
		}
	} else {
		output.PrintInfo(i18n.T("output.executing_down_cmd"))
	}

	if execErr := ExecuteLocalProjectCommand(ctx, projectDir, downCommand, mode); execErr != nil {
		logging.WarnWithContext(ctx, "Failed to execute local project down command", "error", execErr.Error())
		output.PrintError(i18n.T("output.failed_local_down_cmd"))
		return true, execErr
	}

	output.PrintSuccess(i18n.T("output.local_down_cmd_success"))
	return true, nil
}
