package host

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"raioz/internal/domain/models"
	"raioz/internal/logging"
	"raioz/internal/naming"
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
// surface the stderr tail so the user sees why.
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
	deps *models.Deps, serviceName string,
	svc models.Service, projectDir string,
) (*ProcessInfo, error) {
	// Validate that source.command is specified
	if svc.Source.Command == "" {
		return nil, fmt.Errorf("service %s requires 'source.command' field for host execution", serviceName)
	}

	// Get service path
	var servicePath string
	switch svc.Source.Kind {
	case "git":
		servicePath = workspace.GetServicePath(ws, serviceName, svc)
		// Verify path exists
		if _, err := os.Stat(servicePath); os.IsNotExist(err) {
			return nil, fmt.Errorf("service path does not exist: %s", servicePath)
		}
	case "local":
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
	default:
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

	// Synchronous commands live and die with the CLI, so keep them bound to
	// ctx. Daemons must outlive it: cobra's signal context is cancelled on
	// every clean exit, and CommandContext's watchdog would SIGKILL the
	// service the moment `raioz up` / `restart` returns. The settle window
	// below still honors cancellation.
	shouldWait := shouldWaitForCommand(svc.Source.Command)
	var cmd *exec.Cmd
	switch {
	case shouldWait && len(cmdParts) == 1:
		cmd = exec.CommandContext(ctx, cmdParts[0])
	case shouldWait:
		cmd = exec.CommandContext(ctx, cmdParts[0], cmdParts[1:]...)
	case len(cmdParts) == 1:
		cmd = exec.Command(cmdParts[0])
	default:
		cmd = exec.Command(cmdParts[0], cmdParts[1:]...)
	}

	// Set working directory
	cmd.Dir = servicePath

	// Set environment variables (merge with current env)
	cmd.Env = append(os.Environ(), envVars...)

	// Set up output: one combined log per service, at the single path
	// naming.LogFile owns. `up`'s runner writes there and `raioz logs`
	// reads there; this used to write to <workspace>/logs/host/ instead,
	// so the same service changed file depending on which command had
	// launched it last and the idle directory sat frozen with a stale
	// successful startup — the first thing a dev reads when debugging.
	//
	// The project name comes from the same field the reader uses
	// (YAMLProject.ProjectName is cfgDeps.Project.Name), deliberately
	// with no cleverer fallback: any fallback here would be a name the
	// reader can't derive, which is how the two paths diverged.
	projectName := ""
	if deps != nil {
		projectName = deps.Project.Name
	}
	logPath := naming.LogFile(projectName, serviceName)
	if err := os.MkdirAll(filepath.Dir(logPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create logs directory: %w", err)
	}

	logFile, err := os.Create(logPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create log file: %w", err)
	}

	// For synchronous commands (shouldWait), write to both console and log file.
	// For background commands, only write to the log file to avoid cluttering console
	// We'll determine this after checking shouldWait, but set up MultiWriter for both cases
	cmd.Stdout = io.MultiWriter(os.Stdout, logFile)
	cmd.Stderr = io.MultiWriter(os.Stderr, logFile)

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

	// shouldWait was computed above (it decides the exec.Command flavor):
	// "make launch" / "make stop"-style commands complete before continuing;
	// "npm run dev"-style commands run in background and must survive the CLI.
	if shouldWait {
		// For synchronous commands, write to both console and log files
		// Output is already being written to both via MultiWriter set above
		logging.DebugWithContext(
			ctx, "Executing command synchronously (waiting for completion)",
			"service", serviceName, "command", svc.Source.Command,
		)

		if err := cmd.Run(); err != nil {
			// Close the file to ensure output is flushed
			logFile.Close()

			// Build error message (output already shown in console)
			errMsg := fmt.Sprintf("Command failed: %s", svc.Source.Command)
			return nil, fmt.Errorf("%s: %w", errMsg, err)
		}

		// Close the file after successful execution
		logFile.Close()

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

	// For background commands, only write to the log file (not console) to avoid
	// cluttering. Reset stdout/stderr to the log file for background processes
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	// Put the child in its own process group so KillProcessTree (used by
	// restart, down, and tests) can reach grandchildren via Kill(-pid).
	// Without this the orchestrator's host_runner path got it (it sets it
	// inline) but this code path didn't, so restart of a host service
	// silently couldn't kill the previous incarnation.
	SetNewProcessGroup(cmd)

	// Start process in background (not Run, because we want it to run continuously)
	if err := cmd.Start(); err != nil {
		logFile.Close()
		return nil, fmt.Errorf("failed to start process for service %s: %w", serviceName, err)
	}

	// Catch processes that fork+exec ok but die immediately
	// (port already bound, missing config, panic at boot). Only treat a
	// non-zero exit inside the window as a failure — a clean exit 0 is
	// how launchers like `make dev-docker` signal that they
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
			logFile.Close()
			return nil, formatEarlyExitError(serviceName, startSettleWindow, exitErr, logPath)
		case <-ctx.Done():
			// SIGINT/SIGTERM during the settle window: the daemon is no
			// longer bound to ctx (plain exec.Command), so tear its process
			// group down explicitly (mirrors orchestrate.HostRunner.Start).
			_ = KillProcessTree(cmd.Process.Pid)
			logFile.Close()
			return nil, fmt.Errorf(
				"start of service %s cancelled during settle window: %w",
				serviceName, ctx.Err(),
			)
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

	// Close the parent's handle (the child keeps its own dup)
	logFile.Close()

	return processInfo, nil
}
