package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	exectimeout "raioz/internal/exec"
)

// NetworkInfo contains information about a Docker network
type NetworkInfo struct {
	Name       string
	Driver     string
	Scope      string
	External   bool
	CreatedBy   string
}

// EnsureNetwork ensures that a Docker network exists, creating it if necessary
// If the network exists but is not external, it will be reused (idempotent)
func EnsureNetwork(name string) error {
	return EnsureNetworkWithContext(context.Background(), name)
}

// EnsureNetworkWithContext ensures that a Docker network exists, creating it if necessary, with context support
func EnsureNetworkWithContext(ctx context.Context, name string) error {
	// Check if network exists
	exists, info, err := NetworkExistsWithContext(ctx, name)
	if err != nil {
		return fmt.Errorf("failed to check network existence: %w", err)
	}

	if exists {
		// Network exists, verify it's suitable
		if info.External && info.Scope == "local" {
			// External network that exists - perfect
			return nil
		}
		// Network exists but might not be external, that's ok for reuse
		return nil
	}

	// Network doesn't exist, create it
	return CreateNetworkWithContext(ctx, name)
}

// NetworkExists checks if a Docker network exists and returns its info
func NetworkExists(name string) (bool, *NetworkInfo, error) {
	return NetworkExistsWithContext(context.Background(), name)
}

// NetworkExistsWithContext checks if a Docker network exists and returns its info with context support
func NetworkExistsWithContext(ctx context.Context, name string) (bool, *NetworkInfo, error) {
	// Create context with timeout
	timeoutCtx, cancel := exectimeout.WithTimeoutFromContext(ctx, exectimeout.DockerNetworkTimeout)
	defer cancel()

	format := "{{.Name}}|{{.Driver}}|{{.Scope}}|{{.Options}}"
	cmd := exec.CommandContext(timeoutCtx, "docker", "network", "inspect", name, "--format", format)
	output, err := cmd.Output()

	if err != nil {
		// Network doesn't exist
		if exitError, ok := err.(*exec.ExitError); ok {
			if exitError.ExitCode() == 1 {
				// Network not found
				return false, nil, nil
			}
		}
		if exectimeout.IsTimeoutError(timeoutCtx, err) {
			return false, nil, fmt.Errorf("network inspect timed out after %v", exectimeout.DockerNetworkTimeout)
		}
		return false, nil, fmt.Errorf("failed to inspect network: %w", err)
	}

	// Parse output
	parts := strings.Split(strings.TrimSpace(string(output)), "|")
	if len(parts) < 3 {
		return false, nil, fmt.Errorf("unexpected network inspect output format")
	}

	info := &NetworkInfo{
		Name:   parts[0],
		Driver: parts[1],
		Scope:  parts[2],
	}

	// Check if it's external (created outside of compose)
	// Networks created by docker compose have labels, external networks don't
	if len(parts) >= 4 && strings.Contains(parts[3], "com.docker.compose") {
		info.External = false
	} else {
		info.External = true
	}

	return true, info, nil
}

// CreateNetwork creates a new Docker network with bridge driver
func CreateNetwork(name string) error {
	return CreateNetworkWithContext(context.Background(), name)
}

// CreateNetworkWithContext creates a new Docker network with bridge driver with context support
func CreateNetworkWithContext(ctx context.Context, name string) error {
	// Create context with timeout
	timeoutCtx, cancel := exectimeout.WithTimeoutFromContext(ctx, exectimeout.DockerNetworkTimeout)
	defer cancel()

	cmd := exec.CommandContext(timeoutCtx, "docker", "network", "create", "--driver", "bridge", name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if exectimeout.IsTimeoutError(timeoutCtx, err) {
			return fmt.Errorf("network create timed out after %v", exectimeout.DockerNetworkTimeout)
		}
		return fmt.Errorf("failed to create network '%s': %w (output: %s)", name, err, string(output))
	}
	return nil
}

// RemoveNetwork removes a Docker network
func RemoveNetwork(name string) error {
	cmd := exec.Command("docker", "network", "rm", name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// If network is in use, that's ok - we don't want to force remove
		if strings.Contains(string(output), "network is in use") {
			return fmt.Errorf("network '%s' is in use by other containers", name)
		}
		return fmt.Errorf("failed to remove network '%s': %w (output: %s)", name, err, string(output))
	}
	return nil
}

// IsNetworkInUse checks if a network is being used by any containers
func IsNetworkInUse(name string) (bool, error) {
	return IsNetworkInUseWithContext(context.Background(), name)
}

// IsNetworkInUseWithContext checks if a network is being used by any containers with context support
func IsNetworkInUseWithContext(ctx context.Context, name string) (bool, error) {
	// Create context with timeout
	timeoutCtx, cancel := exectimeout.WithTimeoutFromContext(ctx, exectimeout.DockerNetworkTimeout)
	defer cancel()

	cmd := exec.CommandContext(timeoutCtx, "docker", "network", "inspect", name, "--format", "{{len .Containers}}")
	output, err := cmd.Output()
	if err != nil {
		// If network doesn't exist, it's not in use
		if exitError, ok := err.(*exec.ExitError); ok {
			if exitError.ExitCode() == 1 {
				return false, nil
			}
		}
		if exectimeout.IsTimeoutError(timeoutCtx, err) {
			return false, fmt.Errorf("network inspect timed out after %v", exectimeout.DockerNetworkTimeout)
		}
		return false, fmt.Errorf("failed to check network usage: %w", err)
	}

	count := strings.TrimSpace(string(output))
	if count == "0" || count == "" {
		return false, nil
	}

	return true, nil
}

// GetNetworkProjects scans workspace directories to find projects using the network
func GetNetworkProjects(networkName string, baseDir string) ([]string, error) {
	var projects []string
	workspacesDir := filepath.Join(baseDir, "workspaces")

	// Read workspaces directory
	entries, err := os.ReadDir(workspacesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return projects, nil // No workspaces yet
		}
		return nil, fmt.Errorf("failed to read workspaces: %w", err)
	}

	// Check each workspace for state file
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		projectName := entry.Name()
		statePath := filepath.Join(workspacesDir, projectName, ".state.json")

		// Try to load state
		data, err := os.ReadFile(statePath)
		if err != nil {
			continue // Skip if can't read
		}

		// Parse JSON to check network
		var state struct {
			Project struct {
				Network string `json:"network"`
			} `json:"project"`
		}

		if err := json.Unmarshal(data, &state); err != nil {
			continue // Skip if invalid JSON
		}

		if state.Project.Network == networkName {
			projects = append(projects, projectName)
		}
	}

	return projects, nil
}
