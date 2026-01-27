package host

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"raioz/internal/config"
	"raioz/internal/env"
	"raioz/internal/logging"
	"raioz/internal/workspace"
	exectimeout "raioz/internal/exec"
)

// ProcessInfo contains information about a running host process
type ProcessInfo struct {
	PID         int       `json:"pid"`
	Service     string    `json:"service"`
	Command     string    `json:"command"`
	StopCommand string    `json:"stopCommand,omitempty"` // Optional custom stop command
	ComposePath string    `json:"composePath,omitempty"` // Path to docker-compose.yml if service uses docker-compose
	StartTime   time.Time `json:"startTime"`
}

// StartService starts a service directly on the host (without Docker)
// projectDir is the directory where .raioz.json is located (used for local services with path: ".")
func StartService(ctx context.Context, ws *workspace.Workspace, deps *config.Deps, serviceName string, svc config.Service, projectDir string) (*ProcessInfo, error) {
	// Validate that source.command is specified
	if svc.Source.Command == "" {
		return nil, fmt.Errorf("service %s requires 'source.command' field for host execution", serviceName)
	}

	// Get service path
	var servicePath string
	if svc.Source.Kind == "git" {
		servicePath = workspace.GetServicePath(ws, serviceName, svc)
		// Verify path exists
		if _, err := os.Stat(servicePath); os.IsNotExist(err) {
			return nil, fmt.Errorf("service path does not exist: %s", servicePath)
		}
	} else if svc.Source.Kind == "local" {
		// For local services, use the path directly (can be absolute or relative)
		if filepath.IsAbs(svc.Source.Path) {
			servicePath = svc.Source.Path
		} else {
			// Relative path - resolve from project directory (where .raioz.json is)
			// For local services, path "." means the project directory (where .raioz.json is located)
			if svc.Source.Path == "." {
				if projectDir != "" {
					servicePath = projectDir
				} else {
					// Fallback to workspace root if projectDir not provided
					servicePath = ws.Root
				}
			} else {
				// Relative path from project directory
				if projectDir != "" {
					servicePath = filepath.Join(projectDir, svc.Source.Path)
				} else {
					// Fallback to workspace root if projectDir not provided
					servicePath = filepath.Join(ws.Root, svc.Source.Path)
				}
			}
		}
		// Verify path exists
		if _, err := os.Stat(servicePath); os.IsNotExist(err) {
			return nil, fmt.Errorf("service path does not exist: %s", servicePath)
		}
	} else {
		// For image-based services, we can't run them on host (they need to be Docker)
		return nil, fmt.Errorf("image-based services cannot run on host: %s", serviceName)
	}

	// Create symlinks from volumes if specified (for host services)
	if len(svc.Volumes) > 0 {
		if err := createVolumeSymlinks(svc.Volumes, projectDir, servicePath); err != nil {
			return nil, fmt.Errorf("failed to create volume symlinks for service %s: %w", serviceName, err)
		}
	}

	// Resolve environment variables
	envVars, err := resolveEnvVars(ctx, ws, deps, serviceName, svc, projectDir, servicePath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve env vars for service %s: %w", serviceName, err)
	}

	// Parse command (split by spaces, simple parsing)
	cmdParts := parseCommand(svc.Source.Command)
	if len(cmdParts) == 0 {
		return nil, fmt.Errorf("invalid command for service %s: %s", serviceName, svc.Source.Command)
	}

	// Create command
	var cmd *exec.Cmd
	if len(cmdParts) == 1 {
		// Single command (e.g., "npm")
		cmd = exec.CommandContext(ctx, cmdParts[0])
	} else {
		// Command with args (e.g., "npm run dev")
		cmd = exec.CommandContext(ctx, cmdParts[0], cmdParts[1:]...)
	}

	// Set working directory
	cmd.Dir = servicePath

	// Set environment variables (merge with current env)
	cmd.Env = append(os.Environ(), envVars...)

	// Set up output: write to both console and log files
	logDir := filepath.Join(ws.Root, "logs", "host")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create logs directory: %w", err)
	}

	stdoutPath := filepath.Join(logDir, fmt.Sprintf("%s.stdout.log", serviceName))
	stderrPath := filepath.Join(logDir, fmt.Sprintf("%s.stderr.log", serviceName))

	stdoutFile, err := os.Create(stdoutPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout log file: %w", err)
	}
	stderrFile, err := os.Create(stderrPath)
	if err != nil {
		stdoutFile.Close()
		return nil, fmt.Errorf("failed to create stderr log file: %w", err)
	}

	// For synchronous commands (shouldWait), write to both console and log files
	// For background commands, only write to log files to avoid cluttering console
	// We'll determine this after checking shouldWait, but set up MultiWriter for both cases
	cmd.Stdout = io.MultiWriter(os.Stdout, stdoutFile)
	cmd.Stderr = io.MultiWriter(os.Stderr, stderrFile)

	// Detect docker-compose.yml path if service uses docker-compose
	var composePath string
	var explicitComposePath string
	if svc.Commands != nil {
		explicitComposePath = svc.Commands.ComposePath
	}
	composePath = DetectComposePath(servicePath, svc.Source.Command, explicitComposePath)
	if composePath != "" {
		logging.DebugWithContext(ctx, "Detected docker-compose.yml path", "service", serviceName, "composePath", composePath)
	}

	// Check if command should run synchronously (wait for completion)
	// Commands like "make launch" or "make stop" should complete before continuing
	// Commands like "npm run dev" should run in background
	shouldWait := shouldWaitForCommand(svc.Source.Command)

	if shouldWait {
		// For synchronous commands, write to both console and log files
		// Output is already being written to both via MultiWriter set above
		logging.DebugWithContext(ctx, "Executing command synchronously (waiting for completion)", "service", serviceName, "command", svc.Source.Command)

		if err := cmd.Run(); err != nil {
			// Close files to ensure output is flushed
			stdoutFile.Close()
			stderrFile.Close()

			// Build error message (output already shown in console)
			errMsg := fmt.Sprintf("Command failed: %s", svc.Source.Command)
			return nil, fmt.Errorf("%s: %w", errMsg, err)
		}

		// Close files after successful execution
		stdoutFile.Close()
		stderrFile.Close()

		// For synchronous commands, return a dummy ProcessInfo (no PID to track)
		processInfo := &ProcessInfo{
			PID:        0, // No PID to track for synchronous commands
			Service:    serviceName,
			Command:    svc.Source.Command,
			ComposePath: composePath,
			StartTime:  time.Now(),
		}
		return processInfo, nil
	}

	// For background commands, only write to log files (not console) to avoid cluttering
	// Reset stdout/stderr to only log files for background processes
	cmd.Stdout = stdoutFile
	cmd.Stderr = stderrFile

	// Start process in background (not Run, because we want it to run continuously)
	if err := cmd.Start(); err != nil {
		stdoutFile.Close()
		stderrFile.Close()
		return nil, fmt.Errorf("failed to start process for service %s: %w", serviceName, err)
	}

	// Store process info
	processInfo := &ProcessInfo{
		PID:        cmd.Process.Pid,
		Service:    serviceName,
		Command:    svc.Source.Command,
		ComposePath: composePath,
		StartTime:  time.Now(),
	}

	// Close file handles (process will keep them open)
	// Note: In production, you might want to keep these open and manage them differently
	// For now, we close them and let the process inherit them
	stdoutFile.Close()
	stderrFile.Close()

	return processInfo, nil
}

// StopService stops a running host process by PID
func StopService(ctx context.Context, pid int) error {
	return StopServiceWithCommandAndPath(ctx, pid, "", "")
}

// StopServiceWithCommand stops a running host process, optionally using a custom stop command first
// Deprecated: Use StopServiceWithCommandAndPath instead
func StopServiceWithCommand(ctx context.Context, pid int, stopCommand string) error {
	return StopServiceWithCommandAndPath(ctx, pid, stopCommand, "")
}

// StopServiceWithCommandAndPath stops a running host process, optionally using a custom stop command first
// servicePath is the directory where the stop command should be executed
func StopServiceWithCommandAndPath(ctx context.Context, pid int, stopCommand string, servicePath string) error {

	// If a custom stop command is provided, execute it first
	if stopCommand != "" {
		cmdParts := parseCommand(stopCommand)
		if len(cmdParts) > 0 {
			// Use a longer timeout for stop commands (60 seconds)
			stopCtx, cancel := exectimeout.WithTimeoutFromContext(ctx, 60*time.Second)
			defer cancel()

			var cmd *exec.Cmd
			if len(cmdParts) == 1 {
				cmd = exec.CommandContext(stopCtx, cmdParts[0])
			} else {
				cmd = exec.CommandContext(stopCtx, cmdParts[0], cmdParts[1:]...)
			}

			// Set working directory if service path is provided
			if servicePath != "" {
				cmd.Dir = servicePath
				logging.DebugWithContext(ctx, "Executing stop command in service directory", "stopCommand", stopCommand, "servicePath", servicePath, "pid", pid)
			} else {
				logging.DebugWithContext(ctx, "Executing stop command", "stopCommand", stopCommand, "pid", pid)
			}

			// Show output in console for stop commands (they are always synchronous)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr

			// Execute stop command and wait for completion
			// Stop commands like "make stop" should complete before continuing
			if err := cmd.Run(); err != nil {
				// Log but don't fail - fall back to PID kill
				logging.WarnWithContext(ctx, "Stop command failed, falling back to PID kill", "error", err.Error(), "stopCommand", stopCommand)
			} else {
				// Stop command completed successfully
				logging.InfoWithContext(ctx, "Stop command completed successfully", "stopCommand", stopCommand)
				// Check if process is still running (only if we have a valid PID)
				if pid > 0 {
					if running, _ := IsServiceRunning(pid); !running {
						// Process already stopped by the command
						return nil
					}
				} else {
					// No PID to track, command completed successfully
					return nil
				}
			}
		}
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("failed to find process %d: %w", pid, err)
	}

	// Try graceful shutdown first (SIGTERM)
	if err := process.Signal(syscall.SIGTERM); err != nil {
		// Process might already be dead
		if err.Error() == "os: process already finished" {
			return nil
		}
		return fmt.Errorf("failed to send SIGTERM to process %d: %w", pid, err)
	}

	// Wait a bit for graceful shutdown
	timeoutCtx, cancel := exectimeout.WithTimeoutFromContext(ctx, 5*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		_, err := process.Wait()
		done <- err
	}()

	select {
	case <-timeoutCtx.Done():
		// Timeout: force kill
		if err := process.Kill(); err != nil {
			return fmt.Errorf("failed to kill process %d: %w", pid, err)
		}
		return nil
	case err := <-done:
		return err
	}
}

// IsServiceRunning checks if a process is still running
func IsServiceRunning(pid int) (bool, error) {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false, err
	}

	// Send signal 0 to check if process exists
	err = process.Signal(syscall.Signal(0))
	if err == nil {
		return true, nil
	}
	if err.Error() == "os: process already finished" {
		return false, nil
	}
	return false, err
}

// resolveEnvVars resolves environment variables for a host service
func resolveEnvVars(ctx context.Context, ws *workspace.Workspace, deps *config.Deps, serviceName string, svc config.Service, projectDir string, servicePath string) ([]string, error) {
	// Resolve env file path (same logic as Docker)
	envFilePath, err := env.ResolveEnvFileForService(ws, deps, serviceName, svc.Env, projectDir, servicePath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve env file: %w", err)
	}

	var envVars []string
	if envFilePath != "" {
		// Read env file and parse into key=value pairs
		data, err := os.ReadFile(envFilePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read env file: %w", err)
		}

		// Simple parsing: split by lines, skip comments and empty lines
		lines := parseEnvFile(string(data))
		envVars = append(envVars, lines...)
	}

	return envVars, nil
}

// parseCommand parses a command string into command and arguments
// Uses shell-like parsing: splits by spaces, handles quoted strings
func parseCommand(cmdStr string) []string {
	if cmdStr == "" {
		return nil
	}

	// For now, use simple split (can be enhanced later for quoted strings)
	// This works for most common cases: "npm run dev", "go run main.go", etc.
	parts := strings.Fields(cmdStr)
	if len(parts) == 0 {
		return nil
	}

	return parts
}

// createVolumeSymlinks creates symbolic links for host services
// volumes format: ["SRC:DEST", ...] where:
// - SRC is relative to projectDir (or absolute path)
// - DEST is relative to servicePath
func createVolumeSymlinks(volumes []string, projectDir string, servicePath string) error {
	for _, vol := range volumes {
		if vol == "" {
			continue
		}

		// Parse SRC:DEST format
		parts := strings.SplitN(vol, ":", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid volume format '%s': expected 'SRC:DEST'", vol)
		}

		src := strings.TrimSpace(parts[0])
		dest := strings.TrimSpace(parts[1])

		if src == "" || dest == "" {
			return fmt.Errorf("invalid volume format '%s': SRC and DEST cannot be empty", vol)
		}

		// Resolve SRC to absolute path (relative to projectDir if not absolute)
		var srcAbs string
		if filepath.IsAbs(src) {
			srcAbs = src
		} else {
			if projectDir == "" {
				return fmt.Errorf("cannot resolve relative path '%s': projectDir is not provided", src)
			}
			srcAbs = filepath.Join(projectDir, src)
		}

		// Create source path if it doesn't exist (file or directory)
		if _, err := os.Stat(srcAbs); os.IsNotExist(err) {
			// Source doesn't exist, determine if it should be a file or directory
			// Check if destination suggests it's a file (has file extension or doesn't end with common dir patterns)
			destBase := filepath.Base(dest)
			hasExtension := filepath.Ext(destBase) != ""
			isLikelyFile := hasExtension && destBase != "." && destBase != ".."
			
			if isLikelyFile {
				// Likely a file - ensure parent directory exists and create empty file
				parentDir := filepath.Dir(srcAbs)
				if err := os.MkdirAll(parentDir, 0755); err != nil {
					return fmt.Errorf("failed to create parent directory for source file: %w", err)
				}
				// Create empty file
				file, err := os.Create(srcAbs)
				if err != nil {
					return fmt.Errorf("failed to create source file: %w", err)
				}
				file.Close()
			} else {
				// Likely a directory - create directory
				if err := os.MkdirAll(srcAbs, 0755); err != nil {
					return fmt.Errorf("failed to create source directory: %w", err)
				}
			}
		} else if err != nil {
			return fmt.Errorf("failed to check source path: %w", err)
		}

		// Resolve DEST to absolute path (relative to servicePath)
		destAbs := filepath.Join(servicePath, dest)

		// Ensure parent directory of destination exists
		destParent := filepath.Dir(destAbs)
		if err := os.MkdirAll(destParent, 0755); err != nil {
			return fmt.Errorf("failed to create parent directory for destination: %w", err)
		}

		// Check if destination already exists
		if _, err := os.Lstat(destAbs); err == nil {
			// Destination exists, check if it's already a symlink pointing to the same target
			if linkInfo, err := os.Readlink(destAbs); err == nil {
				// It's a symlink, check if it points to the same target
				linkAbs, err := filepath.Abs(filepath.Join(filepath.Dir(destAbs), linkInfo))
				if err == nil {
					srcAbsResolved, err := filepath.Abs(srcAbs)
					if err == nil && linkAbs == srcAbsResolved {
						// Already linked to the same target, skip
						continue
					}
				}
				// Remove existing symlink to recreate it
				if err := os.Remove(destAbs); err != nil {
					return fmt.Errorf("failed to remove existing symlink: %w", err)
				}
			} else {
				// Destination exists but is not a symlink
				return fmt.Errorf("destination path already exists and is not a symlink: %s", destAbs)
			}
		}

		// Resolve source to absolute path for symlink
		srcAbsResolved, err := filepath.Abs(srcAbs)
		if err != nil {
			return fmt.Errorf("failed to resolve absolute path for source: %w", err)
		}

		// Create symlink (use absolute path for source)
		if err := os.Symlink(srcAbsResolved, destAbs); err != nil {
			return fmt.Errorf("failed to create symlink from %s to %s: %w", srcAbsResolved, destAbs, err)
		}
	}

	return nil
}

// shouldWaitForCommand determines if a command should be executed synchronously
// Commands that should wait: make launch, make stop, docker-compose up, scripts, etc.
// Commands that should run in background: npm run dev, go run main.go, etc.
func shouldWaitForCommand(command string) bool {
	// Commands that should complete before continuing
	waitCommands := []string{
		"make launch",
		"make stop",
		"docker-compose up",
		"docker-compose down",
		"docker compose up",
		"docker compose down",
	}

	commandLower := strings.ToLower(command)
	for _, waitCmd := range waitCommands {
		if strings.Contains(commandLower, waitCmd) {
			return true
		}
	}

	// Scripts (installer.sh, setup.sh, etc.) should execute synchronously to catch errors
	// These are typically deployment/setup scripts that should complete before continuing
	if strings.HasSuffix(commandLower, ".sh") || strings.HasPrefix(commandLower, "./") || strings.HasPrefix(commandLower, "sh ") {
		// Exclude long-running scripts that should run in background
		// If it's a simple script execution (not npm run, go run, etc.), wait for it
		if !strings.Contains(commandLower, "npm run") &&
		   !strings.Contains(commandLower, "go run") &&
		   !strings.Contains(commandLower, "python") &&
		   !strings.Contains(commandLower, "node") {
			return true
		}
	}

	// Default: run in background for long-running services
	return false
}

// parseEnvFile parses an env file content into key=value pairs
func parseEnvFile(content string) []string {
	var vars []string
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		// Trim whitespace
		line = strings.TrimSpace(line)

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Add to vars (assumes format: KEY=VALUE)
		vars = append(vars, line)
	}

	return vars
}
