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

			// Build command: docker compose -f <path> up -d --remove-orphans [service1 service2 ...]
			// --remove-orphans cleans up containers from services no longer in the compose file
			// (e.g., after a Replace operation in a shared workspace)
			args := []string{"compose", "-f", composePath, "up", "-d", "--remove-orphans"}
			if len(serviceNames) > 0 {
				args = append(args, serviceNames...)
			}

			cmd := exec.CommandContext(timeoutCtx, runtime.Binary(), args...)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr

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

			args := []string{"compose", "-f", composePath, "restart"}
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

			args := []string{"compose", "-f", composePath, "up", "-d", "--force-recreate", "--no-deps"}
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
	if _, err := os.Stat(composePath); os.IsNotExist(err) {
		return nil
	}
	if err := ValidateComposePath(composePath); err != nil {
		return fmt.Errorf("invalid compose path: %w", err)
	}
	timeoutCtx, cancel := exectimeout.WithTimeoutFromContext(ctx, exectimeout.DockerComposeDownTimeout)
	defer cancel()
	// Stop the service (leave other services running)
	cmd := exec.CommandContext(timeoutCtx, runtime.Binary(), "compose", "-f", composePath, "stop", serviceName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return exectimeout.HandleTimeoutError(timeoutCtx, err, "docker compose stop", exectimeout.DockerComposeDownTimeout)
	}
	// Remove the container so the local project can recreate it
	rmCtx, rmCancel := exectimeout.WithTimeoutFromContext(ctx, exectimeout.DockerComposeDownTimeout)
	defer rmCancel()
	rmCmd := exec.CommandContext(rmCtx, runtime.Binary(), "compose", "-f", composePath, "rm", "-f", serviceName)
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
	// Check if compose file exists
	if _, err := os.Stat(composePath); os.IsNotExist(err) {
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

			cmd := exec.CommandContext(timeoutCtx, runtime.Binary(), "compose", "-f", composePath, "down")
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr

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

	if _, err := os.Stat(composePath); os.IsNotExist(err) {
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
	cmd := exec.CommandContext(timeoutCtx, runtime.Binary(), "compose", "-f", composePath, "ps", "--services", "--status", "running")
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
	cmd2 := exec.CommandContext(timeoutCtx, runtime.Binary(), "compose", "-f", composePath, "ps", "--services")
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
