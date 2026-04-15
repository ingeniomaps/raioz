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
	"raioz/internal/runtime"
)

const (
	// DefaultHealthCheckTimeout is the maximum time to wait for services to become healthy
	DefaultHealthCheckTimeout = 5 * time.Minute
	// HealthCheckInterval is the interval between health checks
	HealthCheckInterval = 2 * time.Second
)

// WaitForServicesHealthy waits for all services and infra to become healthy
// Returns error if timeout is reached or context is cancelled
func WaitForServicesHealthy(
	ctx context.Context, composePath string,
	serviceNames []string, infraNames []string,
	projectName string,
) error {
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
			return fmt.Errorf("health check cancelled: %w", ctx.Err())
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
			return fmt.Errorf("health check cancelled: %w", ctx.Err())
		case <-time.After(HealthCheckInterval):
			// Continue checking
		}
	}
}

// isServiceHealthyWithContext checks if a service is healthy
func isServiceHealthyWithContext(
	ctx context.Context, composePath string,
	serviceName string, _ string,
) (bool, error) {
	// Get container name
	containerName, err := GetContainerNameWithContext(ctx, composePath, serviceName)
	if err != nil || containerName == "" {
		return false, nil // Service not running
	}

	// Create context with timeout for inspect
	timeoutCtx, cancel := exectimeout.WithTimeoutFromContext(ctx, exectimeout.DockerInspectTimeout)
	defer cancel()

	// Get container inspect data — single call reused for all checks below
	cmd := exec.CommandContext(timeoutCtx, runtime.Binary(), "inspect", containerName)
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

	// If no health check is configured, try to detect service type and use custom checks.
	// Pass the already-fetched inspect data to avoid redundant docker calls.
	healthy, err := checkServiceReadinessFromInspect(ctx, containerName, &inspect)
	if err == nil {
		return healthy, nil
	}

	// Fallback: if we can't determine readiness, wait a bit and check if container is running
	// This is less reliable but better than assuming it's ready immediately
	// For critical services like databases, we should have health checks configured
	return false, nil // Don't consider healthy without health check or custom readiness check
}

// checkServiceReadinessFromInspect checks if a service is ready using
// already-fetched inspect data. This avoids redundant docker inspect calls
// that the previous implementation (checkServiceReadinessWithoutHealthcheck)
// was making.
func checkServiceReadinessFromInspect(
	ctx context.Context, containerName string,
	inspect *ContainerInspect,
) (bool, error) {
	image := inspect.Config.Image

	// Extract environment variables from the already-fetched inspect data
	envVars := make(map[string]string)
	for _, env := range inspect.Config.Env {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 {
			envVars[parts[0]] = parts[1]
		}
	}

	// Check if it's a postgres container and try pg_isready
	if strings.Contains(image, "postgres") {
		pgUser := envVars["POSTGRES_USER"]
		if pgUser == "" {
			pgUser = "postgres"
		}
		pgDb := envVars["POSTGRES_DB"]
		if pgDb == "" {
			pgDb = pgUser
		}

		timeoutCtx, cancel := exectimeout.WithTimeoutFromContext(ctx, exectimeout.DockerInspectTimeout)
		defer cancel()

		pgCmd := exec.CommandContext(
			timeoutCtx, runtime.Binary(), "exec", containerName,
			"pg_isready", "-U", pgUser, "-d", pgDb,
		)
		if err := pgCmd.Run(); err == nil {
			return true, nil
		}
		return false, nil
	}

	// For other services without health checks, we can't reliably determine readiness
	return false, fmt.Errorf("no custom readiness check available for %s", containerName)
}
