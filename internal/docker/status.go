package docker

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	exectimeout "raioz/internal/exec"
)

// AreServicesRunning checks if all required services are already running
func AreServicesRunning(composePath string, serviceNames []string) (bool, error) {
	if _, err := os.Stat(composePath); os.IsNotExist(err) {
		return false, nil
	}

	statuses, err := GetServicesStatus(composePath)
	if err != nil {
		return false, fmt.Errorf("failed to get services status: %w", err)
	}

	// Check if all services are running
	for _, name := range serviceNames {
		if status, ok := statuses[name]; !ok || status != "running" {
			return false, nil
		}
	}

	return true, nil
}

// GetServiceNames extracts all service and infra names from compose
func GetServiceNames(composePath string) ([]string, error) {
	return GetServiceNamesWithContext(context.Background(), composePath)
}

// GetServiceNamesWithContext extracts all service and infra names from compose with context support
func GetServiceNamesWithContext(ctx context.Context, composePath string) ([]string, error) {
	if _, err := os.Stat(composePath); os.IsNotExist(err) {
		return []string{}, nil
	}

	// Validate path to prevent command injection
	if err := ValidateComposePath(composePath); err != nil {
		return nil, fmt.Errorf("invalid compose path: %w", err)
	}

	// Create context with timeout
	timeoutCtx, cancel := exectimeout.WithTimeoutFromContext(ctx, exectimeout.DockerStatusTimeout)
	defer cancel()

	// Use docker compose config to get service names
	cmd := exec.CommandContext(timeoutCtx, "docker", "compose", "-f", composePath, "config", "--services")
	output, err := cmd.Output()
	if err != nil {
		if exectimeout.IsTimeoutError(timeoutCtx, err) {
			return nil, fmt.Errorf("docker compose config timed out after %v", exectimeout.DockerStatusTimeout)
		}
		return nil, fmt.Errorf("failed to get service names: %w", err)
	}

	var names []string
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			names = append(names, line)
		}
	}

	return names, nil
}
