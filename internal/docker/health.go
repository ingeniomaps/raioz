package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	exectimeout "raioz/internal/exec"
	"raioz/internal/output"
)

const (
	// DefaultHealthCheckTimeout is the maximum time to wait for services to become healthy
	DefaultHealthCheckTimeout = 5 * time.Minute
	// HealthCheckInterval is the interval between health checks
	HealthCheckInterval = 2 * time.Second
)

// WaitForServicesHealthy waits for all services and infra to become healthy
// Returns error if timeout is reached or context is cancelled
func WaitForServicesHealthy(ctx context.Context, composePath string, serviceNames []string, infraNames []string, projectName string) error {
	allNames := append([]string{}, serviceNames...)
	allNames = append(allNames, infraNames...)

	if len(allNames) == 0 {
		return nil // No services to wait for
	}

	// Create context with timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, DefaultHealthCheckTimeout)
	defer cancel()

	output.PrintProgress(fmt.Sprintf("Waiting for %d service(s) to become healthy...", len(allNames)))

	startTime := time.Now()
	for {
		// Check if context is cancelled or timeout reached
		select {
		case <-timeoutCtx.Done():
			return fmt.Errorf("timeout waiting for services to become healthy after %v", DefaultHealthCheckTimeout)
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		allHealthy := true
		for _, name := range allNames {
			// Get service health status
			healthy, err := isServiceHealthyWithContext(timeoutCtx, composePath, name, projectName)
			if err != nil {
				// Log error but continue checking
				allHealthy = false
				continue
			}
			if !healthy {
				allHealthy = false
				break
			}
		}

		if allHealthy {
			duration := time.Since(startTime)
			output.PrintProgressDone(fmt.Sprintf("All services are healthy (took %v)", duration.Round(time.Second)))
			return nil
		}

		// Wait before next check
		select {
		case <-timeoutCtx.Done():
			return fmt.Errorf("timeout waiting for services to become healthy after %v", DefaultHealthCheckTimeout)
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(HealthCheckInterval):
			// Continue checking
		}
	}
}

// isServiceHealthyWithContext checks if a service is healthy
func isServiceHealthyWithContext(ctx context.Context, composePath string, serviceName string, projectName string) (bool, error) {
	// Get container name
	containerName, err := GetContainerNameWithContext(ctx, composePath, serviceName)
	if err != nil || containerName == "" {
		return false, nil // Service not running
	}

	// Create context with timeout for inspect
	timeoutCtx, cancel := exectimeout.WithTimeoutFromContext(ctx, exectimeout.DockerInspectTimeout)
	defer cancel()

	// Get container inspect data
	cmd := exec.CommandContext(timeoutCtx, "docker", "inspect", containerName)
	output, err := cmd.Output()
	if err != nil {
		return false, nil // Service not healthy if inspect fails
	}

	var inspectData []ContainerInspect
	if err := json.Unmarshal(output, &inspectData); err != nil || len(inspectData) == 0 {
		return false, nil
	}

	inspect := inspectData[0]

	// Check if container is running
	if inspect.State.Status != "running" {
		return false, nil
	}

	// Check health status if health check is configured
	if inspect.State.Health != nil {
		healthStatus := strings.ToLower(inspect.State.Health.Status)
		// Only "healthy" means the service is ready
		return healthStatus == "healthy", nil
	}

	// If no health check is configured, try to detect service type and use custom checks
	// This is especially important for databases that need to be ready before project commands run
	healthy, err := checkServiceReadinessWithoutHealthcheck(ctx, composePath, serviceName, projectName)
	if err == nil {
		return healthy, nil
	}

	// Fallback: if we can't determine readiness, wait a bit and check if container is running
	// This is less reliable but better than assuming it's ready immediately
	// For critical services like databases, we should have health checks configured
	return false, nil // Don't consider healthy without health check or custom readiness check
}

// checkServiceReadinessWithoutHealthcheck checks if a service is ready even without healthcheck
// This is used for services that don't have health checks configured but need to be ready
func checkServiceReadinessWithoutHealthcheck(ctx context.Context, composePath string, serviceName string, projectName string) (bool, error) {
	// Get container name
	containerName, err := GetContainerNameWithContext(ctx, composePath, serviceName)
	if err != nil || containerName == "" {
		return false, nil // Service not running
	}

	// Create context with timeout for inspect
	timeoutCtx, cancel := exectimeout.WithTimeoutFromContext(ctx, exectimeout.DockerInspectTimeout)
	defer cancel()

	// Get container inspect data to check image and env vars
	cmd := exec.CommandContext(timeoutCtx, "docker", "inspect", containerName)
	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("failed to inspect container: %w", err)
	}

	var inspectData []ContainerInspect
	if err := json.Unmarshal(output, &inspectData); err != nil || len(inspectData) == 0 {
		return false, fmt.Errorf("failed to parse inspect data")
	}

	inspect := inspectData[0]
	image := inspect.Config.Image

	// Extract environment variables
	envVars := make(map[string]string)
	for _, env := range inspect.Config.Env {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 {
			envVars[parts[0]] = parts[1]
		}
	}

	// Check if it's a postgres container and try pg_isready
	if strings.Contains(image, "postgres") {
		// Get postgres user and database from env vars
		pgUser := envVars["POSTGRES_USER"]
		if pgUser == "" {
			pgUser = "postgres" // Default
		}
		pgDb := envVars["POSTGRES_DB"]
		if pgDb == "" {
			pgDb = pgUser // Default to same as user
		}

		// Try to execute pg_isready inside the container
		pgCmd := exec.CommandContext(timeoutCtx, "docker", "exec", containerName, "pg_isready", "-U", pgUser, "-d", pgDb)
		if err := pgCmd.Run(); err == nil {
			return true, nil
		}
		// If pg_isready fails, service is not ready
		return false, nil
	}

	// For other services without health checks, we can't reliably determine readiness
	// Return error to indicate we can't check
	return false, fmt.Errorf("no health check and no custom readiness check available for service %s", serviceName)
}
