package docker

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	exectimeout "raioz/internal/exec"
	"raioz/internal/runtime"
)

// VolumeType represents the type of volume
type VolumeType string

const (
	VolumeTypeNamed   VolumeType = "named"   // Named volume: mongo-data:/data/db
	VolumeTypeBind    VolumeType = "bind"    // Bind mount: ./path:/container/path
	VolumeTypeAnonymous VolumeType = "anonymous" // Anonymous: /container/path
)

// VolumeInfo contains information about a parsed volume
type VolumeInfo struct {
	Type         VolumeType
	Source       string // For named: volume name, for bind: host path
	Destination  string // Container path
	Original     string // Original volume string
}

// ParseVolume parses a volume string and determines its type
// Examples:
//   - "mongo-data:/data/db" -> named volume
//   - "./data:/app/data" -> bind mount
//   - "/host/path:/container/path" -> bind mount
//   - "/container/path" -> anonymous volume
func ParseVolume(volume string) (*VolumeInfo, error) {
	if volume == "" {
		return nil, fmt.Errorf("empty volume string")
	}

	parts := strings.SplitN(volume, ":", 2)

	// If no colon, it's an anonymous volume
	if len(parts) == 1 {
		return &VolumeInfo{
			Type:        VolumeTypeAnonymous,
			Source:      "",
			Destination: parts[0],
			Original:    volume,
		}, nil
	}

	source := parts[0]
	dest := parts[1]

	// Check if source is a named volume (no slash, or starts with / but is a volume name)
	// Named volumes typically don't start with . or / (absolute paths)
	isNamed := !strings.HasPrefix(source, ".") &&
		!strings.HasPrefix(source, "/") &&
		!strings.Contains(source, string(filepath.Separator))

	if isNamed {
		// Additional check: if it contains only alphanumeric, dashes, underscores
		// and doesn't look like a path, it's likely a named volume
		if !strings.Contains(source, "../") && !strings.Contains(source, "./") {
			return &VolumeInfo{
				Type:        VolumeTypeNamed,
				Source:      source,
				Destination: dest,
				Original:    volume,
			}, nil
		}
	}

	// It's a bind mount
	return &VolumeInfo{
		Type:        VolumeTypeBind,
		Source:      source,
		Destination: dest,
		Original:    volume,
	}, nil
}

// ExtractNamedVolumes extracts all named volumes from a list of volume strings
func ExtractNamedVolumes(volumes []string) ([]string, error) {
	namedVolumes := make(map[string]bool)

	for _, vol := range volumes {
		if vol == "" {
			continue
		}

		info, err := ParseVolume(vol)
		if err != nil {
			return nil, fmt.Errorf("failed to parse volume '%s': %w", vol, err)
		}

		if info.Type == VolumeTypeNamed {
			namedVolumes[info.Source] = true
		}
	}

	// Convert map to slice
	result := make([]string, 0, len(namedVolumes))
	for vol := range namedVolumes {
		result = append(result, vol)
	}

	return result, nil
}

// expandTilde expands leading ~ to the current user's home directory.
// ~/path -> $HOME/path; ~ -> $HOME. Paths without ~ are returned unchanged.
func expandTilde(path string) (string, error) {
	if path == "" || path[0] != '~' {
		return path, nil
	}
	if len(path) == 1 || path[1] == '/' {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get home directory for ~ expansion: %w", err)
		}
		if len(path) == 1 {
			return home, nil
		}
		return filepath.Join(home, path[2:]), nil
	}
	// ~user not supported, return as-is
	return path, nil
}

// ResolveRelativeVolumes converts relative paths in bind mount volumes to absolute paths
// based on the project directory. Paths starting with ~ are expanded to the user's home.
// Named volumes and anonymous volumes are left unchanged.
func ResolveRelativeVolumes(volumes []string, projectDir string) ([]string, error) {
	resolved := make([]string, 0, len(volumes))

	for _, vol := range volumes {
		if vol == "" {
			continue
		}

		info, err := ParseVolume(vol)
		if err != nil {
			return nil, fmt.Errorf("failed to parse volume '%s': %w", vol, err)
		}

		// Only resolve relative paths for bind mounts
		if info.Type == VolumeTypeBind {
			source := info.Source
			// Expand ~ to user home so the path is independent of project location
			if strings.HasPrefix(source, "~") {
				expanded, err := expandTilde(source)
				if err != nil {
					return nil, fmt.Errorf("volume source '%s': %w", source, err)
				}
				source = expanded
			}
			var absPath string
			if filepath.IsAbs(source) {
				absPath = source
			} else if source != "" {
				var err error
				absPath, err = filepath.Abs(filepath.Join(projectDir, info.Source))
				if err != nil {
					return nil, fmt.Errorf("failed to resolve relative path '%s': %w", info.Source, err)
				}
			} else {
				resolved = append(resolved, vol)
				continue
			}
			dest := info.Destination
			var modeSuffix string
			if strings.HasSuffix(dest, ":ro") {
				dest = strings.TrimSuffix(dest, ":ro")
				modeSuffix = ":ro"
			} else if strings.HasSuffix(dest, ":rw") {
				dest = strings.TrimSuffix(dest, ":rw")
				modeSuffix = ":rw"
			}
			resolved = append(resolved, absPath+":"+dest+modeSuffix)
		} else {
			// Named volumes and anonymous volumes are left unchanged
			resolved = append(resolved, vol)
		}
	}

	return resolved, nil
}

// NormalizeVolumeNamesInStrings normalizes volume names in volume strings with project prefix
// Replaces original volume names with normalized names (project_volume_name)
func NormalizeVolumeNamesInStrings(volumes []string, projectName string, volumeMap map[string]string) ([]string, error) {
	normalized := make([]string, 0, len(volumes))

	for _, vol := range volumes {
		if vol == "" {
			continue
		}

		info, err := ParseVolume(vol)
		if err != nil {
			return nil, fmt.Errorf("failed to parse volume '%s': %w", vol, err)
		}

		// Only normalize named volumes
		if info.Type == VolumeTypeNamed {
			normalizedName, exists := volumeMap[info.Source]
			if !exists {
				// Normalize the volume name
				normalizedName, err = NormalizeVolumeName(projectName, info.Source)
				if err != nil {
					return nil, fmt.Errorf("failed to normalize volume name '%s': %w", info.Source, err)
				}
				volumeMap[info.Source] = normalizedName
			}
			// Replace the volume name in the string
			normalized = append(normalized, normalizedName+":"+info.Destination)
		} else {
			// Keep bind mounts and anonymous volumes as is
			normalized = append(normalized, vol)
		}
	}

	return normalized, nil
}

// VolumeExists checks if a named Docker volume exists
func VolumeExists(name string) (bool, error) {
	return VolumeExistsWithContext(context.Background(), name)
}

// VolumeExistsWithContext checks if a named Docker volume exists with context support
func VolumeExistsWithContext(ctx context.Context, name string) (bool, error) {
	// Create context with timeout
	timeoutCtx, cancel := exectimeout.WithTimeoutFromContext(ctx, exectimeout.DockerVolumeTimeout)
	defer cancel()

	cmd := exec.CommandContext(timeoutCtx, runtime.Binary(), "volume", "inspect", name)
	err := cmd.Run()

	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			if exitError.ExitCode() == 1 {
				// Volume not found
				return false, nil
			}
		}
		if exectimeout.IsTimeoutError(timeoutCtx, err) {
			return false, fmt.Errorf("volume inspect timed out after %v", exectimeout.DockerVolumeTimeout)
		}
		return false, fmt.Errorf("failed to inspect volume: %w", err)
	}

	return true, nil
}

// EnsureVolume ensures that a named volume exists, creating it if necessary
func EnsureVolume(name string) error {
	return EnsureVolumeWithContext(context.Background(), name)
}

// EnsureVolumeWithContext ensures that a named volume exists, creating it if necessary, with context support
func EnsureVolumeWithContext(ctx context.Context, name string) error {
	exists, err := VolumeExistsWithContext(ctx, name)
	if err != nil {
		return err
	}

	if exists {
		return nil // Volume already exists
	}

	// Create context with timeout
	timeoutCtx, cancel := exectimeout.WithTimeoutFromContext(ctx, exectimeout.DockerVolumeTimeout)
	defer cancel()

	// Create volume
	cmd := exec.CommandContext(timeoutCtx, runtime.Binary(), "volume", "create", name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if exectimeout.IsTimeoutError(timeoutCtx, err) {
			return fmt.Errorf("volume create timed out after %v", exectimeout.DockerVolumeTimeout)
		}
		return fmt.Errorf("failed to create volume '%s': %w (output: %s)", name, err, string(output))
	}

	return nil
}

// RemoveVolume removes a named Docker volume
func RemoveVolume(name string) error {
	return RemoveVolumeWithContext(context.Background(), name)
}

// RemoveVolumeWithContext removes a named Docker volume with context support
func RemoveVolumeWithContext(ctx context.Context, name string) error {
	// Check if volume exists first
	exists, err := VolumeExistsWithContext(ctx, name)
	if err != nil {
		return fmt.Errorf("failed to check if volume exists: %w", err)
	}

	if !exists {
		// Volume doesn't exist, nothing to remove
		return nil
	}

	// Create context with timeout
	timeoutCtx, cancel := exectimeout.WithTimeoutFromContext(ctx, exectimeout.DockerVolumeTimeout)
	defer cancel()

	// Remove volume
	cmd := exec.CommandContext(timeoutCtx, runtime.Binary(), "volume", "rm", name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if exectimeout.IsTimeoutError(timeoutCtx, err) {
			return fmt.Errorf("volume remove timed out after %v", exectimeout.DockerVolumeTimeout)
		}
		// Check if volume is in use
		if strings.Contains(string(output), "in use") || strings.Contains(string(output), "is being used") {
			return fmt.Errorf("volume '%s' is in use and cannot be removed", name)
		}
		return fmt.Errorf("failed to remove volume '%s': %w (output: %s)", name, err, string(output))
	}

	return nil
}

// GetVolumeProjects finds projects that might be using a named volume
