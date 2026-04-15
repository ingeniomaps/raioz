package docker

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	exectimeout "raioz/internal/exec"
	"raioz/internal/resilience"
	"raioz/internal/runtime"
)

func Up(composePath string) error {
	return UpWithContext(context.Background(), composePath)
}

// UpWithContext starts Docker Compose services with context support
func UpWithContext(ctx context.Context, composePath string) error {
	return UpServicesWithContext(ctx, composePath, nil)
}

// UpServicesWithContext starts specific Docker Compose services with context support
// If serviceNames is nil or empty, starts all services
func UpServicesWithContext(ctx context.Context, composePath string, serviceNames []string) error {
	// Validate path to prevent command injection
	if err := ValidateComposePath(composePath); err != nil {
		return fmt.Errorf("invalid compose path: %w", err)
	}

	// Use circuit breaker and retry logic for docker compose up
	dockerCB := resilience.GetDockerCircuitBreaker()
	retryConfig := resilience.DockerRetryConfig()

	operationName := "docker compose up"
	if len(serviceNames) > 0 {
		operationName = fmt.Sprintf("docker compose up %v", serviceNames)
	}

	return resilience.RetryWithContext(ctx, retryConfig, operationName, func(ctx context.Context) error {
		return dockerCB.ExecuteWithContext(ctx, operationName, func(ctx context.Context) error {
			// Create context with timeout
			timeoutCtx, cancel := exectimeout.WithTimeoutFromContext(ctx, exectimeout.DockerComposeUpTimeout)
			defer cancel()

			// Build command: docker compose -f <path> [ -f <path2> ... ] up -d --remove-orphans [services...]
			// --remove-orphans cleans up containers from services no longer in the compose file
			// (e.g., after a Replace operation in a shared workspace). When the caller
			// sets an explicit project name via WithComposeProjectName, it is exported
			// as COMPOSE_PROJECT_NAME so --remove-orphans only affects containers in
			// that project, not anything sharing the directory basename.
			// `--env-file` flags must appear BEFORE the subcommand (docker
			// compose parses them as top-level flags), which is why we
			// prepend them instead of appending. The context-plumbed list
			// is typically populated for dependencies declared with
			// `compose:` whose user-supplied fragment uses ${VAR}
			// interpolation from an external .env file.
			args := append([]string{"compose"}, ComposeEnvFileArgs(timeoutCtx)...)
			args = append(args, ComposeFileArgs(composePath)...)
			args = append(args, "up", "-d", "--remove-orphans")
			if len(serviceNames) > 0 {
				args = append(args, serviceNames...)
			}

			cmd := exec.CommandContext(timeoutCtx, runtime.Binary(), args...)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			cmd.Env = composeCommandEnv(timeoutCtx)

			err := cmd.Run()
			return exectimeout.HandleTimeoutError(timeoutCtx, err, operationName, exectimeout.DockerComposeUpTimeout)
		})
	})
}

// RestartServicesWithContext restarts specific Docker Compose services
func RestartServicesWithContext(ctx context.Context, composePath string, serviceNames []string) error {
	if err := ValidateComposePath(composePath); err != nil {
		return fmt.Errorf("invalid compose path: %w", err)
	}

	dockerCB := resilience.GetDockerCircuitBreaker()
	retryConfig := resilience.DockerRetryConfig()
	operationName := fmt.Sprintf("docker compose restart %v", serviceNames)

	return resilience.RetryWithContext(ctx, retryConfig, operationName, func(ctx context.Context) error {
		return dockerCB.ExecuteWithContext(ctx, operationName, func(ctx context.Context) error {
			timeoutCtx, cancel := exectimeout.WithTimeoutFromContext(ctx, exectimeout.DockerComposeUpTimeout)
			defer cancel()

			args := append([]string{"compose"}, ComposeFileArgs(composePath)...)
			args = append(args, "restart")
			args = append(args, serviceNames...)

			cmd := exec.CommandContext(timeoutCtx, runtime.Binary(), args...)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr

			err := cmd.Run()
			return exectimeout.HandleTimeoutError(timeoutCtx, err, operationName, exectimeout.DockerComposeUpTimeout)
		})
	})
}

// ForceRecreateServicesWithContext recreates containers (docker compose up -d --force-recreate)
func ForceRecreateServicesWithContext(ctx context.Context, composePath string, serviceNames []string) error {
	if err := ValidateComposePath(composePath); err != nil {
		return fmt.Errorf("invalid compose path: %w", err)
	}

	dockerCB := resilience.GetDockerCircuitBreaker()
	retryConfig := resilience.DockerRetryConfig()
	operationName := fmt.Sprintf("docker compose up --force-recreate %v", serviceNames)

	return resilience.RetryWithContext(ctx, retryConfig, operationName, func(ctx context.Context) error {
		return dockerCB.ExecuteWithContext(ctx, operationName, func(ctx context.Context) error {
			timeoutCtx, cancel := exectimeout.WithTimeoutFromContext(ctx, exectimeout.DockerComposeUpTimeout)
			defer cancel()

			args := append([]string{"compose"}, ComposeFileArgs(composePath)...)
			args = append(args, "up", "-d", "--force-recreate", "--no-deps")
			args = append(args, serviceNames...)

			cmd := exec.CommandContext(timeoutCtx, runtime.Binary(), args...)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr

			err := cmd.Run()
			return exectimeout.HandleTimeoutError(timeoutCtx, err, operationName, exectimeout.DockerComposeUpTimeout)
		})
	})
}

func Down(composePath string) error {
	return DownWithContext(context.Background(), composePath)
}

// StopServiceWithContext stops and removes only one service from a compose project.
// Use this to resolve a service conflict without affecting other services or infra.
func StopServiceWithContext(ctx context.Context, composePath string, serviceName string) error {
	if serviceName == "" {
		return nil
	}
	if _, err := os.Stat(PrimaryComposeFile(composePath)); os.IsNotExist(err) {
		return nil
	}
	if err := ValidateComposePath(composePath); err != nil {
		return fmt.Errorf("invalid compose path: %w", err)
	}
	timeoutCtx, cancel := exectimeout.WithTimeoutFromContext(ctx, exectimeout.DockerComposeDownTimeout)
	defer cancel()
	// Stop the service (leave other services running)
	stopArgs := append([]string{"compose"}, ComposeFileArgs(composePath)...)
	stopArgs = append(stopArgs, "stop", serviceName)
	cmd := exec.CommandContext(timeoutCtx, runtime.Binary(), stopArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return exectimeout.HandleTimeoutError(timeoutCtx, err, "docker compose stop", exectimeout.DockerComposeDownTimeout)
	}
	// Remove the container so the local project can recreate it
	rmCtx, rmCancel := exectimeout.WithTimeoutFromContext(ctx, exectimeout.DockerComposeDownTimeout)
	defer rmCancel()
	rmArgs := append([]string{"compose"}, ComposeFileArgs(composePath)...)
	rmArgs = append(rmArgs, "rm", "-f", serviceName)
	rmCmd := exec.CommandContext(rmCtx, runtime.Binary(), rmArgs...)
	rmCmd.Stdout = os.Stdout
	rmCmd.Stderr = os.Stderr
	if err := rmCmd.Run(); err != nil {
		// rm may fail if container already removed; non-fatal
		return nil
	}
	return nil
}

// DownWithContext stops Docker Compose services with context support
func DownWithContext(ctx context.Context, composePath string) error {
	// Check if compose file exists (use first file as probe)
	if _, err := os.Stat(PrimaryComposeFile(composePath)); os.IsNotExist(err) {
		return nil // Already down
	}

	// Validate path to prevent command injection
	if err := ValidateComposePath(composePath); err != nil {
		return fmt.Errorf("invalid compose path: %w", err)
	}

	// Use circuit breaker and retry logic for docker compose down
	dockerCB := resilience.GetDockerCircuitBreaker()
	retryConfig := resilience.DockerRetryConfig()

	return resilience.RetryWithContext(ctx, retryConfig, "docker compose down", func(ctx context.Context) error {
		return dockerCB.ExecuteWithContext(ctx, "docker compose down", func(ctx context.Context) error {
			// Create context with timeout
			timeoutCtx, cancel := exectimeout.WithTimeoutFromContext(ctx, exectimeout.DockerComposeDownTimeout)
			defer cancel()

			downArgs := append([]string{"compose"}, ComposeEnvFileArgs(timeoutCtx)...)
			downArgs = append(downArgs, ComposeFileArgs(composePath)...)
			downArgs = append(downArgs, "down")
			cmd := exec.CommandContext(timeoutCtx, runtime.Binary(), downArgs...)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			cmd.Env = composeCommandEnv(timeoutCtx)

			err := cmd.Run()
			return exectimeout.HandleTimeoutError(timeoutCtx, err, "docker compose down", exectimeout.DockerComposeDownTimeout)
		})
	})
}

func GetServicesStatus(composePath string) (map[string]string, error) {
	return GetServicesStatusWithContext(context.Background(), composePath)
}

// GetServicesStatusWithContext gets Docker Compose services status with context support
func GetServicesStatusWithContext(ctx context.Context, composePath string) (map[string]string, error) {
	status := make(map[string]string)

	if _, err := os.Stat(PrimaryComposeFile(composePath)); os.IsNotExist(err) {
		return status, nil
	}

	// Validate path to prevent command injection
	if err := ValidateComposePath(composePath); err != nil {
		return status, fmt.Errorf("invalid compose path: %w", err)
	}

	// Create context with timeout
	timeoutCtx, cancel := exectimeout.WithTimeoutFromContext(ctx, exectimeout.DockerStatusTimeout)
	defer cancel()

	// Get running services
	psRunArgs := append([]string{"compose"}, ComposeFileArgs(composePath)...)
	psRunArgs = append(psRunArgs, "ps", "--services", "--status", "running")
	cmd := exec.CommandContext(timeoutCtx, runtime.Binary(), psRunArgs...)
	if proj := composeProjectEnvFromContext(timeoutCtx); proj != "" {
		cmd.Env = append(os.Environ(), proj)
	}
	output, err := cmd.Output()
	if err != nil {
		if exectimeout.IsTimeoutError(timeoutCtx, err) {
			return status, exectimeout.HandleTimeoutError(timeoutCtx, err, "docker compose ps", exectimeout.DockerStatusTimeout)
		}
		return status, nil // Non-timeout errors are ignored (service might not be running)
	}

	// Parse output
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			status[line] = "running"
		}
	}

	// Get all services (running and stopped)
	psAllArgs := append([]string{"compose"}, ComposeFileArgs(composePath)...)
	psAllArgs = append(psAllArgs, "ps", "--services")
	cmd2 := exec.CommandContext(timeoutCtx, runtime.Binary(), psAllArgs...)
	if proj := composeProjectEnvFromContext(timeoutCtx); proj != "" {
		cmd2.Env = append(os.Environ(), proj)
	}
	output2, err := cmd2.Output()
	if err == nil {
		allLines := strings.Split(string(output2), "\n")
		for _, line := range allLines {
			line = strings.TrimSpace(line)
			if line != "" {
				if _, exists := status[line]; !exists {
					status[line] = "stopped"
				}
			}
		}
	} else if exectimeout.IsTimeoutError(timeoutCtx, err) {
		return status, exectimeout.HandleTimeoutError(timeoutCtx, err, "docker compose ps", exectimeout.DockerStatusTimeout)
	}

	return status, nil
}

// StopContainerWithContext stops a Docker container by name (docker stop name).
// Returns nil if the container was stopped or if it does not exist; returns error for other failures.
func StopContainerWithContext(ctx context.Context, containerName string) error {
	if containerName == "" {
		return nil
	}
	cmd := exec.CommandContext(ctx, runtime.Binary(), "stop", containerName)
	out, err := cmd.CombinedOutput()
	if err != nil {
		if strings.Contains(string(out), "No such container") || strings.Contains(string(out), "is not running") {
			return nil
		}
		return fmt.Errorf("docker stop %s: %w", containerName, err)
	}
	if len(out) > 0 {
		os.Stdout.Write(out)
	}
	return nil
}

// ListContainersByLabels returns the names of all containers (running or
// stopped) that match ALL provided label=value filters. Returns an empty
// slice on any docker error; callers should treat a miss as "nothing to do"
// rather than a fatal failure.
func ListContainersByLabels(ctx context.Context, labels map[string]string) []string {
	if len(labels) == 0 {
		return nil
	}
	args := []string{"ps", "-a", "--format", "{{.Names}}"}
	for k, v := range labels {
		args = append(args, "--filter", fmt.Sprintf("label=%s=%s", k, v))
	}
	cmd := exec.CommandContext(ctx, runtime.Binary(), args...)
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	raw := strings.TrimSpace(string(out))
	if raw == "" {
		return nil
	}
	var names []string
	for _, line := range strings.Split(raw, "\n") {
		if n := strings.TrimSpace(line); n != "" {
			names = append(names, n)
		}
	}
	return names
}
