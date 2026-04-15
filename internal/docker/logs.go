package docker

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	exectimeout "raioz/internal/exec"
	"raioz/internal/runtime"
)

// LogsOptions contains options for viewing logs
type LogsOptions struct {
	Follow   bool
	Tail     int
	Services []string
}

// ViewLogs displays logs for services using docker compose logs
func ViewLogs(composePath string, opts LogsOptions) error {
	return ViewLogsWithContext(context.Background(), composePath, opts)
}

// ViewLogsWithContext displays logs for services using docker compose logs with context support
func ViewLogsWithContext(ctx context.Context, composePath string, opts LogsOptions) error {
	// Check if compose file exists
	if _, err := os.Stat(composePath); os.IsNotExist(err) {
		return fmt.Errorf("compose file not found: %s (project may not be running)", composePath)
	}

	// Validate path to prevent command injection
	if err := ValidateComposePath(composePath); err != nil {
		return fmt.Errorf("invalid compose path: %w", err)
	}

	// Create context with timeout (only if not following, as follow mode runs indefinitely)
	var timeoutCtx context.Context
	if opts.Follow {
		// For follow mode, use the context as-is (no timeout, user can cancel with Ctrl+C)
		timeoutCtx = ctx
	} else {
		var cancel context.CancelFunc
		timeoutCtx, cancel = exectimeout.WithTimeoutFromContext(ctx, exectimeout.DockerLogsTimeout)
		defer cancel()
	}

	// Build docker compose logs command
	args := append([]string{"compose"}, ComposeFileArgs(composePath)...)
	args = append(args, "logs")

	// Add follow flag if specified
	if opts.Follow {
		args = append(args, "--follow")
	}

	// Add tail flag if specified
	if opts.Tail > 0 {
		args = append(args, "--tail", strconv.Itoa(opts.Tail))
	} else if !opts.Follow {
		// Default tail if not following (show last 100 lines)
		args = append(args, "--tail", "100")
	}

	// Add service names if specified
	if len(opts.Services) > 0 {
		args = append(args, opts.Services...)
	}

	// Execute command
	cmd := exec.CommandContext(timeoutCtx, runtime.Binary(), args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if !opts.Follow {
		return exectimeout.HandleTimeoutError(timeoutCtx, err, "docker compose logs", exectimeout.DockerLogsTimeout)
	}
	return err
}

// GetAvailableServices returns list of available services from compose
func GetAvailableServices(composePath string) ([]string, error) {
	return GetAvailableServicesWithContext(context.Background(), composePath)
}

// GetAvailableServicesWithContext returns list of available services from compose with context support
func GetAvailableServicesWithContext(ctx context.Context, composePath string) ([]string, error) {
	if _, err := os.Stat(PrimaryComposeFile(composePath)); os.IsNotExist(err) {
		return []string{}, nil
	}

	// Validate path to prevent command injection
	if err := ValidateComposePath(composePath); err != nil {
		return nil, fmt.Errorf("invalid compose path: %w", err)
	}

	// Create context with timeout
	timeoutCtx, cancel := exectimeout.WithTimeoutFromContext(ctx, exectimeout.DockerStatusTimeout)
	defer cancel()

	configArgs := append([]string{"compose"}, ComposeFileArgs(composePath)...)
	configArgs = append(configArgs, "config", "--services")
	cmd := exec.CommandContext(timeoutCtx, runtime.Binary(), configArgs...)
	output, err := cmd.Output()
	if err != nil {
		if exectimeout.IsTimeoutError(timeoutCtx, err) {
			return nil, fmt.Errorf("docker compose config timed out after %v", exectimeout.DockerStatusTimeout)
		}
		return nil, fmt.Errorf("failed to get services: %w", err)
	}

	var services []string
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			services = append(services, line)
		}
	}

	return services, nil
}
