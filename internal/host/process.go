package host

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"raioz/internal/config"
	"raioz/internal/logging"
	"raioz/internal/workspace"
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

// startSettleWindow is how long StartService waits after a successful
// cmd.Start() to make sure the process did not die immediately. If the
// process exits inside this window we treat the start as a failure and
// surface the stderr tail so the user sees why (issue 008).
//
// Background: cmd.Start() returns nil for any process that fork+exec'd
// successfully — even if it crashes 5 ms later (port already bound,
// missing config, etc). Without this guard `raioz status` then reports
// "running" while the service is already dead.
//
// Exposed as a package var (not a const) so tests can shrink it.
var startSettleWindow = 500 * time.Millisecond

// StartService starts a service directly on the host (without Docker)
// projectDir is the directory where .raioz.json is located (used for local services with path: ".")
func StartService(
	ctx context.Context, ws *workspace.Workspace,
	deps *config.Deps, serviceName string,
	svc config.Service, projectDir string,
) (*ProcessInfo, error) {
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
		logging.DebugWithContext(
			ctx, "Executing command synchronously (waiting for completion)",
			"service", serviceName, "command", svc.Source.Command,
		)

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
			PID:         0, // No PID to track for synchronous commands
			Service:     serviceName,
			Command:     svc.Source.Command,
			ComposePath: composePath,
			StartTime:   time.Now(),
		}
		return processInfo, nil
	}

	// For background commands, only write to log files (not console) to avoid cluttering
	// Reset stdout/stderr to only log files for background processes
	cmd.Stdout = stdoutFile
	cmd.Stderr = stderrFile

	// Put the child in its own process group so KillProcessTree (used by
	// restart, down, and tests) can reach grandchildren via Kill(-pid).
	// Without this the orchestrator's host_runner path got it (it sets it
	// inline) but this code path didn't, so restart of a host service
	// silently couldn't kill the previous incarnation. Issue 013.
	SetNewProcessGroup(cmd)

	// Start process in background (not Run, because we want it to run continuously)
	if err := cmd.Start(); err != nil {
		stdoutFile.Close()
		stderrFile.Close()
		return nil, fmt.Errorf("failed to start process for service %s: %w", serviceName, err)
	}

	// Issue 008: catch processes that fork+exec ok but die immediately
	// (port already bound, missing config, panic at boot). Only treat a
	// non-zero exit inside the window as a failure — a clean exit 0 is
	// how launchers like `make dev-docker` (issue 010) signal that they
	// detached a long-running container and completed successfully.
	if startSettleWindow > 0 {
		waitErr := make(chan error, 1)
		go func() { waitErr <- cmd.Wait() }()

		select {
		case exitErr := <-waitErr:
			if exitErr == nil {
				// Clean detach. Continue with the existing flow — PID
				// stays recorded but downstream logic (status_host,
				// proxy.target) is what makes the service observable.
				break
			}
			stdoutFile.Close()
			stderrFile.Close()
			return nil, formatEarlyExitError(serviceName, startSettleWindow, exitErr, stderrPath)
		case <-time.After(startSettleWindow):
			// Process is still alive past the settle window. The wait
			// goroutine remains parked on cmd.Wait() and writes to the
			// buffered channel when the process eventually exits — that
			// drain is harmless and avoids zombies.
		}
	}

	// Store process info
	processInfo := &ProcessInfo{
		PID:         cmd.Process.Pid,
		Service:     serviceName,
		Command:     svc.Source.Command,
		ComposePath: composePath,
		StartTime:   time.Now(),
	}

	// Close file handles (process will keep them open)
	// Note: In production, you might want to keep these open and manage them differently
	// For now, we close them and let the process inherit them
	stdoutFile.Close()
	stderrFile.Close()

	return processInfo, nil
}
