package host

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"raioz/internal/config"
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
