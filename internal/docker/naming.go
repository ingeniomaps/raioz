package docker

import (
	"fmt"
	"regexp"
	"strings"
)

const (
	// MaxContainerNameLength is the maximum length for Docker container names
	MaxContainerNameLength = 63
	// MaxNetworkNameLength is the maximum length for Docker network names
	MaxNetworkNameLength = 63
	// MaxVolumeNameLength is the maximum length for Docker volume names
	MaxVolumeNameLength = 255
)

var (
	// InvalidCharsRegex matches characters that are not allowed in Docker names
	InvalidCharsRegex = regexp.MustCompile(`[^a-z0-9-]`)
	// MultipleDashesRegex matches multiple consecutive dashes
	MultipleDashesRegex = regexp.MustCompile(`-+`)
)

// NormalizeName normalizes a name for use in Docker resources
// It converts to lowercase, replaces invalid characters with dashes,
// removes duplicate dashes, and truncates if necessary
func NormalizeName(parts ...string) (string, error) {
	if len(parts) == 0 {
		return "", fmt.Errorf("at least one part is required")
	}

	// Join parts with dashes
	name := strings.Join(parts, "-")

	// Convert to lowercase
	name = strings.ToLower(name)

	// Replace invalid characters with dashes
	name = InvalidCharsRegex.ReplaceAllString(name, "-")

	// Remove duplicate dashes
	name = MultipleDashesRegex.ReplaceAllString(name, "-")

	// Remove leading and trailing dashes
	name = strings.Trim(name, "-")

	// Validate that name is not empty
	if name == "" {
		return "", fmt.Errorf("normalized name is empty")
	}

	return name, nil
}

// NormalizeContainerName normalizes a container name
// Format: {workspace}-{service} if workspace is explicitly set (different from project), otherwise raioz-{project}-{service}
// Maximum length: 63 characters (Docker limit)
// workspace: workspace name from GetWorkspaceName() (could be explicit workspace or project name)
// project: project name (used to determine if workspace was explicitly set)
// hasExplicitWorkspace: true if workspace field was explicitly set in config, false otherwise
func NormalizeContainerName(workspace, service string, project string, hasExplicitWorkspace bool) (string, error) {
	var name string
	var err error

	// If no explicit workspace was set, use raioz-{project}-{service}
	// Otherwise, use {workspace}-{service}
	if !hasExplicitWorkspace {
		// No explicit workspace, use raioz-{project}-{service} format
		name, err = NormalizeName("raioz", project, service)
		if err != nil {
			return "", err
		}

		// Truncate if necessary
		if len(name) > MaxContainerNameLength {
			prefix := fmt.Sprintf("raioz-%s-", project)
			if len(prefix) >= MaxContainerNameLength {
				name = name[:MaxContainerNameLength]
			} else {
				maxServiceLen := MaxContainerNameLength - len(prefix)
				serviceNormalized, _ := NormalizeName(service)
				if len(serviceNormalized) > maxServiceLen {
					serviceNormalized = serviceNormalized[:maxServiceLen]
				}
				name = prefix + serviceNormalized
			}
			name = strings.TrimSuffix(name, "-")
		}
	} else {
		// Explicit workspace set, use {workspace}-{service} format
		name, err = NormalizeName(workspace, service)
		if err != nil {
			return "", err
		}

		// Truncate if necessary
		if len(name) > MaxContainerNameLength {
			prefixWithDash := fmt.Sprintf("%s-", workspace)
			if len(prefixWithDash) >= MaxContainerNameLength {
				name = name[:MaxContainerNameLength]
			} else {
				maxServiceLen := MaxContainerNameLength - len(prefixWithDash)
				serviceNormalized, _ := NormalizeName(service)
				if len(serviceNormalized) > maxServiceLen {
					serviceNormalized = serviceNormalized[:maxServiceLen]
				}
				name = prefixWithDash + serviceNormalized
			}
			name = strings.TrimSuffix(name, "-")
		}
	}

	return name, nil
}

// NormalizeInfraName normalizes an infra resource name
// Format: {workspace}-{infra} if workspace is explicitly set, otherwise raioz-{project}-{infra}
func NormalizeInfraName(workspace, infra string, project string, hasExplicitWorkspace bool) (string, error) {
	return NormalizeContainerName(workspace, infra, project, hasExplicitWorkspace)
}

// NormalizeNetworkName normalizes a network name
// Networks already use network (root level), but we can add prefix if needed
// For now, we keep the existing format but normalize it
func NormalizeNetworkName(networkName string) (string, error) {
	name, err := NormalizeName(networkName)
	if err != nil {
		return "", err
	}

	// Truncate if necessary
	if len(name) > MaxNetworkNameLength {
		name = name[:MaxNetworkNameLength]
		name = strings.TrimSuffix(name, "-")
	}

	return name, nil
}

// ValidateName validates that a name follows Docker naming conventions
func ValidateName(name string, maxLength int) error {
	if name == "" {
		return fmt.Errorf("name cannot be empty")
	}

	if len(name) > maxLength {
		return fmt.Errorf("name exceeds maximum length of %d characters: %d", maxLength, len(name))
	}

	// Check for invalid characters (only lowercase letters, numbers, and dashes allowed)
	if InvalidCharsRegex.MatchString(name) {
		return fmt.Errorf("name contains invalid characters (only lowercase letters, numbers, and dashes allowed): %s", name)
	}

	// Check for leading/trailing dashes
	if strings.HasPrefix(name, "-") || strings.HasSuffix(name, "-") {
		return fmt.Errorf("name cannot start or end with a dash: %s", name)
	}

	// Check for consecutive dashes
	if MultipleDashesRegex.MatchString(name) && strings.Count(name, "--") > 0 {
		return fmt.Errorf("name cannot contain consecutive dashes: %s", name)
	}

	return nil
}

// ValidateContainerName validates a container name
func ValidateContainerName(name string) error {
	return ValidateName(name, MaxContainerNameLength)
}

// ValidateNetworkName validates a network name
func ValidateNetworkName(name string) error {
	return ValidateName(name, MaxNetworkNameLength)
}

// ValidateVolumeName validates a volume name
func ValidateVolumeName(name string) error {
	// Volume names can be longer than container/network names
	return ValidateName(name, MaxVolumeNameLength)
}

// NormalizeVolumeName normalizes a volume name with the project prefix
// Format: {project}_{volume_name} (using underscore as separator)
// Maximum length: 255 characters (Docker limit)
func NormalizeVolumeName(project, volumeName string) (string, error) {
	// Normalize project name (lowercase, valid chars only)
	projectNormalized, err := NormalizeName(project)
	if err != nil {
		return "", fmt.Errorf("failed to normalize project name: %w", err)
	}

	// Normalize volume name (lowercase, valid chars only, but preserve underscores)
	// Convert to lowercase and replace invalid chars, but keep underscores and dashes
	volumeLower := strings.ToLower(volumeName)
	// Replace invalid chars (but preserve underscores and dashes which are valid)
	volumeNormalized := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return '-'
	}, volumeLower)
	// Remove multiple consecutive dashes (but keep underscores)
	volumeNormalized = MultipleDashesRegex.ReplaceAllString(volumeNormalized, "-")
	volumeNormalized = strings.Trim(volumeNormalized, "-")

	if volumeNormalized == "" {
		return "", fmt.Errorf("normalized volume name is empty")
	}

	// Check if volume name already has the project prefix to avoid duplicates
	prefix := projectNormalized + "_"
	if strings.HasPrefix(volumeNormalized, prefix) {
		// Already has prefix, return as is
		return volumeNormalized, nil
	}

	// Combine with underscore separator
	name := prefix + volumeNormalized

	// Truncate if necessary (Docker limit is 255 characters)
	if len(name) > MaxVolumeNameLength {
		// Calculate how much we need to truncate
		// Keep project prefix, truncate volume name
		if len(prefix) >= MaxVolumeNameLength {
			// Even prefix is too long, truncate everything
			name = name[:MaxVolumeNameLength]
		} else {
			// Truncate volume name to fit
			maxVolumeLen := MaxVolumeNameLength - len(prefix)
			if len(volumeNormalized) > maxVolumeLen {
				volumeNormalized = volumeNormalized[:maxVolumeLen]
			}
			name = prefix + volumeNormalized
		}
		// Remove trailing underscore if any
		name = strings.TrimSuffix(name, "_")
	}

	return name, nil
}
