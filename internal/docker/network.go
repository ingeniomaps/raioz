package docker

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"raioz/internal/config"
	exectimeout "raioz/internal/exec"
	"raioz/internal/runtime"
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
	return EnsureNetworkWithConfig(NetworkConfig{Name: name}, false)
}

// EnsureNetworkWithConfig ensures that a Docker network exists, creating it if necessary
// If askConfirmation is true, prompts the user before creating the network
func EnsureNetworkWithConfig(config NetworkConfig, askConfirmation bool) error {
	return EnsureNetworkWithConfigAndContext(context.Background(), config, askConfirmation)
}

// EnsureNetworkWithContext ensures that a Docker network exists, creating it if necessary, with context support
func EnsureNetworkWithContext(ctx context.Context, name string) error {
	return EnsureNetworkWithConfigAndContext(ctx, NetworkConfig{Name: name}, false)
}

// EnsureNetworkWithConfigAndContext ensures that a Docker network exists, creating it if necessary, with context support
// If askConfirmation is true, prompts the user before creating the network
func EnsureNetworkWithConfigAndContext(ctx context.Context, config NetworkConfig, askConfirmation bool) error {
	// Check if network exists
	exists, info, err := NetworkExistsWithContext(ctx, config.Name)
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
		// If subnet was specified but network exists, we don't modify it (as per requirements)
		return nil
	}

	// Network doesn't exist, ask for confirmation if requested
	if askConfirmation {
		confirmed, err := askNetworkCreationConfirmation(config)
		if err != nil {
			return fmt.Errorf("failed to get user confirmation: %w", err)
		}
		if !confirmed {
			return fmt.Errorf("network creation cancelled by user")
		}
	}

	// Create the network
	return CreateNetworkWithConfigAndContext(ctx, config, false)
}

// askNetworkCreationConfirmation prompts the user to confirm network creation
func askNetworkCreationConfirmation(config NetworkConfig) (bool, error) {
	fmt.Printf("\n⚠️  Network '%s' does not exist.\n", config.Name)
	if config.Subnet != "" {
		fmt.Printf("   Subnet: %s\n", config.Subnet)
	}
	fmt.Print("Do you want to create it? (yes/no): ")

	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false, fmt.Errorf("failed to read user response: %w", err)
	}

	response = strings.TrimSpace(strings.ToLower(response))
	return response == "yes" || response == "y", nil
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
	cmd := exec.CommandContext(timeoutCtx, runtime.Binary(), "network", "inspect", name, "--format", format)
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

// NetworkConfig contains network creation parameters
type NetworkConfig struct {
	Name   string // Network name
	Subnet string // Optional subnet in CIDR notation (e.g., "150.150.0.0/16")
}

// CreateNetwork creates a new Docker network with bridge driver
func CreateNetwork(name string) error {
	return CreateNetworkWithConfig(NetworkConfig{Name: name}, false)
}

// CreateNetworkWithConfig creates a new Docker network with optional subnet
// If askConfirmation is true, prompts the user before creating the network
func CreateNetworkWithConfig(config NetworkConfig, askConfirmation bool) error {
	return CreateNetworkWithConfigAndContext(context.Background(), config, askConfirmation)
}

// CreateNetworkWithContext creates a new Docker network with bridge driver with context support
func CreateNetworkWithContext(ctx context.Context, name string) error {
	return CreateNetworkWithConfigAndContext(ctx, NetworkConfig{Name: name}, false)
}

// CreateNetworkWithConfigAndContext creates a new Docker network with optional subnet and context support
// If askConfirmation is true, prompts the user before creating the network
func CreateNetworkWithConfigAndContext(ctx context.Context, config NetworkConfig, askConfirmation bool) error {
	// Create context with timeout
	timeoutCtx, cancel := exectimeout.WithTimeoutFromContext(ctx, exectimeout.DockerNetworkTimeout)
	defer cancel()

	// Build docker network create command
	args := []string{"network", "create", "--driver", "bridge"}

	// Add subnet if specified
	if config.Subnet != "" {
		args = append(args, "--subnet", config.Subnet)
	}

	args = append(args, config.Name)

	cmd := exec.CommandContext(timeoutCtx, runtime.Binary(), args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if exectimeout.IsTimeoutError(timeoutCtx, err) {
			return fmt.Errorf("network create timed out after %v", exectimeout.DockerNetworkTimeout)
		}
		return fmt.Errorf("failed to create network '%s': %w (output: %s)", config.Name, err, string(output))
	}
	return nil
}

// RemoveNetwork removes a Docker network
func RemoveNetwork(name string) error {
	cmd := exec.Command(runtime.Binary(), "network", "rm", name)
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

	cmd := exec.CommandContext(timeoutCtx, runtime.Binary(), "network", "inspect", name, "--format", "{{len .Containers}}")
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
			Network config.NetworkConfig `json:"network"`
		}

		if err := json.Unmarshal(data, &state); err != nil {
			continue // Skip if invalid JSON
		}

		if state.Network.GetName() == networkName {
			projects = append(projects, projectName)
		}
	}

	return projects, nil
}

// ConnectContainerToNetwork attaches a running container to a Docker network.
// If the container is already connected, this is a no-op.
func ConnectContainerToNetwork(ctx context.Context, containerName, networkName string, aliases []string) error {
	timeoutCtx, cancel := exectimeout.WithTimeoutFromContext(ctx, exectimeout.DockerNetworkTimeout)
	defer cancel()

	args := []string{"network", "connect"}
	for _, alias := range aliases {
		args = append(args, "--alias", alias)
	}
	args = append(args, networkName, containerName)

	cmd := exec.CommandContext(timeoutCtx, runtime.Binary(), args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Already connected is not an error
		if strings.Contains(string(output), "already exists") {
			return nil
		}
		if exectimeout.IsTimeoutError(timeoutCtx, err) {
			return fmt.Errorf("network connect timed out after %v", exectimeout.DockerNetworkTimeout)
		}
		return fmt.Errorf("failed to connect %s to network %s: %w (%s)",
			containerName, networkName, err, string(output))
	}
	return nil
}
