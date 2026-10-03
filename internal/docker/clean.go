package docker

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	exectimeout "raioz/internal/exec"
	"raioz/internal/naming"
	"raioz/internal/runtime"
)

// CleanupOptions contains options for cleanup operations
type CleanupOptions struct {
	DryRun   bool
	All      bool
	Images   bool
	Volumes  bool
	Networks bool
	Force    bool
}

// CleanProjectWithContext cleans up stopped services and resources for a project with context support
func CleanProjectWithContext(ctx context.Context, composePath string, dryRun bool) ([]string, error) {
	var actions []string

	if _, err := os.Stat(composePath); os.IsNotExist(err) {
		return actions, nil // Already clean
	}

	// Validate path to prevent command injection
	if err := ValidateComposePath(composePath); err != nil {
		return actions, fmt.Errorf("invalid compose path: %w", err)
	}

	if dryRun {
		actions = append(actions, fmt.Sprintf("Would remove compose file: %s", composePath))
		return actions, nil
	}

	// Create context with timeout
	timeoutCtx, cancel := exectimeout.WithTimeoutFromContext(ctx, exectimeout.DockerComposeDownTimeout)
	defer cancel()

	// Remove stopped containers, networks, and images created by compose
	cmd := exec.CommandContext(timeoutCtx, runtime.Binary(), "compose", "-f", composePath, "down", "--remove-orphans")
	output, err := cmd.CombinedOutput()
	if err != nil {
		if exectimeout.IsTimeoutError(timeoutCtx, err) {
			return actions, fmt.Errorf("docker compose down timed out after %v", exectimeout.DockerComposeDownTimeout)
		}
		return actions, fmt.Errorf("failed to clean compose: %w (output: %s)", err, string(output))
	}

	// Remove compose file
	if err := os.Remove(composePath); err != nil && !os.IsNotExist(err) {
		return actions, fmt.Errorf("failed to remove compose file: %w", err)
	}

	actions = append(actions, fmt.Sprintf("Cleaned compose file: %s", composePath))
	return actions, nil
}

type cleanScopeKey struct{}

// WithCleanScope narrows what the CleanUnused* helpers may remove to the
// resources carrying every one of these labels. The helpers never act
// without a scope: with none given they fall back to "anything raioz
// created", and never to the whole daemon — a prune that also takes the
// volumes and networks of unrelated stacks is not a raioz clean.
func WithCleanScope(ctx context.Context, labels map[string]string) context.Context {
	return context.WithValue(ctx, cleanScopeKey{}, labels)
}

// cleanFilterArgs returns the `--filter label=k=v` arguments for the
// scope in ctx, key-sorted so the command line is deterministic.
func cleanFilterArgs(ctx context.Context) []string {
	labels, _ := ctx.Value(cleanScopeKey{}).(map[string]string)
	if len(labels) == 0 {
		labels = map[string]string{naming.LabelManaged: "true"}
	}
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	args := make([]string, 0, len(keys)*2)
	for _, k := range keys {
		args = append(args, "--filter", "label="+k+"="+labels[k])
	}
	return args
}

// CleanUnusedImagesWithContext removes unused Docker images with context support
func CleanUnusedImagesWithContext(ctx context.Context, dryRun bool) ([]string, error) {
	var actions []string

	// Create context with timeout
	timeoutCtx, cancel := exectimeout.WithTimeoutFromContext(ctx, exectimeout.DockerInspectTimeout)
	defer cancel()

	if dryRun {
		// List unused images
		listArgs := append([]string{"images", "--filter", "dangling=true", "-q"}, cleanFilterArgs(ctx)...)
		cmd := exec.CommandContext(timeoutCtx, runtime.Binary(), listArgs...)
		output, err := cmd.Output()
		if err != nil {
			if exectimeout.IsTimeoutError(timeoutCtx, err) {
				return actions, fmt.Errorf("docker images list timed out after %v", exectimeout.DockerInspectTimeout)
			}
			return actions, fmt.Errorf("failed to list unused images: %w", err)
		}

		images := strings.Split(strings.TrimSpace(string(output)), "\n")
		for _, img := range images {
			if img != "" {
				actions = append(actions, fmt.Sprintf("Would remove image: %s", img))
			}
		}

		if len(images) == 0 || images[0] == "" {
			actions = append(actions, "No unused images found")
		}

		return actions, nil
	}

	// Remove unused images
	pruneArgs := append([]string{"image", "prune", "-f"}, cleanFilterArgs(ctx)...)
	cmd := exec.CommandContext(timeoutCtx, runtime.Binary(), pruneArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if exectimeout.IsTimeoutError(timeoutCtx, err) {
			return actions, fmt.Errorf("docker image prune timed out after %v", exectimeout.DockerInspectTimeout)
		}
		return actions, fmt.Errorf("failed to prune images: %w (output: %s)", err, string(output))
	}

	actions = append(actions, "Removed unused images")
	return actions, nil
}

// CleanUnusedVolumesWithContext lists or removes unused Docker volumes with context support
func CleanUnusedVolumesWithContext(ctx context.Context, dryRun bool, force bool) ([]string, error) {
	var actions []string

	// Create context with timeout
	timeoutCtx, cancel := exectimeout.WithTimeoutFromContext(ctx, exectimeout.DockerVolumeTimeout)
	defer cancel()

	if dryRun {
		// List unused volumes
		listArgs := append([]string{"volume", "ls", "-q", "-f", "dangling=true"}, cleanFilterArgs(ctx)...)
		cmd := exec.CommandContext(timeoutCtx, runtime.Binary(), listArgs...)
		output, err := cmd.Output()
		if err != nil {
			if exectimeout.IsTimeoutError(timeoutCtx, err) {
				return actions, fmt.Errorf("docker volume ls timed out after %v", exectimeout.DockerVolumeTimeout)
			}
			return actions, fmt.Errorf("failed to list unused volumes: %w", err)
		}

		volumes := strings.Split(strings.TrimSpace(string(output)), "\n")
		for _, vol := range volumes {
			if vol != "" {
				actions = append(actions, fmt.Sprintf("Would remove volume: %s", vol))
			}
		}

		if len(volumes) == 0 || volumes[0] == "" {
			actions = append(actions, "No unused volumes found")
		}

		return actions, nil
	}

	if !force {
		return actions, fmt.Errorf("--force required to remove volumes (use --force flag)")
	}

	// Remove unused volumes
	// --all: raioz's volumes are named, and a plain prune only takes
	// anonymous ones. The label scope is what keeps this from being the
	// daemon-wide prune it would otherwise be.
	pruneArgs := append([]string{"volume", "prune", "-f", "--all"}, cleanFilterArgs(ctx)...)
	cmd := exec.CommandContext(timeoutCtx, runtime.Binary(), pruneArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if exectimeout.IsTimeoutError(timeoutCtx, err) {
			return actions, fmt.Errorf("docker volume prune timed out after %v", exectimeout.DockerVolumeTimeout)
		}
		return actions, fmt.Errorf("failed to prune volumes: %w (output: %s)", err, string(output))
	}

	actions = append(actions, "Removed unused volumes")
	return actions, nil
}

// CleanUnusedNetworksWithContext removes unused Docker networks with context support
func CleanUnusedNetworksWithContext(ctx context.Context, dryRun bool) ([]string, error) {
	var actions []string

	// Create context with timeout
	timeoutCtx, cancel := exectimeout.WithTimeoutFromContext(ctx, exectimeout.DockerNetworkTimeout)
	defer cancel()

	if dryRun {
		// List unused networks (not raioz-* networks that might be in use)
		listArgs := append([]string{"network", "ls", "-q", "-f", "dangling=true"}, cleanFilterArgs(ctx)...)
		cmd := exec.CommandContext(timeoutCtx, runtime.Binary(), listArgs...)
		output, err := cmd.Output()
		if err != nil {
			if exectimeout.IsTimeoutError(timeoutCtx, err) {
				return actions, fmt.Errorf("docker network ls timed out after %v", exectimeout.DockerNetworkTimeout)
			}
			return actions, fmt.Errorf("failed to list unused networks: %w", err)
		}

		networks := strings.Split(strings.TrimSpace(string(output)), "\n")
		for _, net := range networks {
			if net != "" {
				// Get network name
				cmd2 := exec.CommandContext(timeoutCtx, runtime.Binary(), "network", "inspect", "-f", "{{.Name}}", net)
				nameOutput, err := cmd2.Output()
				if err == nil {
					netName := strings.TrimSpace(string(nameOutput))
					actions = append(actions, fmt.Sprintf("Would remove network: %s", netName))
				}
			}
		}

		if len(networks) == 0 || networks[0] == "" {
			actions = append(actions, "No unused networks found")
		}

		return actions, nil
	}

	// Remove unused networks
	pruneArgs := append([]string{"network", "prune", "-f"}, cleanFilterArgs(ctx)...)
	cmd := exec.CommandContext(timeoutCtx, runtime.Binary(), pruneArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if exectimeout.IsTimeoutError(timeoutCtx, err) {
			return actions, fmt.Errorf("docker network prune timed out after %v", exectimeout.DockerNetworkTimeout)
		}
		return actions, fmt.Errorf("failed to prune networks: %w (output: %s)", err, string(output))
	}

	actions = append(actions, "Removed unused networks")
	return actions, nil
}

// GetAllProjectWorkspaces returns all project workspace directories
func GetAllProjectWorkspaces(baseDir string) ([]string, error) {
	workspacesDir := filepath.Join(baseDir, "workspaces")
	if _, err := os.Stat(workspacesDir); os.IsNotExist(err) {
		return []string{}, nil
	}

	entries, err := os.ReadDir(workspacesDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read workspaces directory: %w", err)
	}

	var workspaces []string
	for _, entry := range entries {
		if entry.IsDir() {
			workspaces = append(workspaces, filepath.Join(workspacesDir, entry.Name()))
		}
	}

	return workspaces, nil
}

// CleanAllProjectsWithContext cleans all stopped projects with context support
func CleanAllProjectsWithContext(ctx context.Context, baseDir string, dryRun bool) ([]string, error) {
	var actions []string

	workspaces, err := GetAllProjectWorkspaces(baseDir)
	if err != nil {
		return actions, fmt.Errorf("failed to get project workspaces: %w", err)
	}

	if len(workspaces) == 0 {
		actions = append(actions, "No projects found to clean")
		return actions, nil
	}

	for _, ws := range workspaces {
		composePath := filepath.Join(ws, "docker-compose.generated.yml")
		projectActions, err := CleanProjectWithContext(ctx, composePath, dryRun)
		if err != nil {
			// Log error but continue with other projects
			actions = append(actions, fmt.Sprintf("⚠️  Error cleaning %s: %v", ws, err))
			continue
		}
		actions = append(actions, projectActions...)

		// Remove state file if exists
		statePath := filepath.Join(ws, ".state.json")
		if _, err := os.Stat(statePath); err == nil {
			if dryRun {
				actions = append(actions, fmt.Sprintf("Would remove state file: %s", statePath))
			} else {
				if err := os.Remove(statePath); err != nil {
					actions = append(actions, fmt.Sprintf("⚠️  Failed to remove state file %s: %v", statePath, err))
				} else {
					actions = append(actions, fmt.Sprintf("Removed state file: %s", statePath))
				}
			}
		}
	}

	return actions, nil
}
