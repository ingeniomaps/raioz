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

// NormalizeContainerName normalizes a container name with the raioz prefix
// Format: raioz-{project}-{service}
// Maximum length: 63 characters (Docker limit)
func NormalizeContainerName(project, service string) (string, error) {
	name, err := NormalizeName("raioz", project, service)
	if err != nil {
		return "", err
	}

	// Truncate if necessary (Docker limit is 63 characters)
	if len(name) > MaxContainerNameLength {
		// Calculate how much we need to truncate
		// Keep "raioz-" prefix and project name, truncate service name
		prefix := fmt.Sprintf("raioz-%s-", project)
		if len(prefix) >= MaxContainerNameLength {
			// Even prefix is too long, truncate everything
			name = name[:MaxContainerNameLength]
		} else {
			// Truncate service name to fit
			maxServiceLen := MaxContainerNameLength - len(prefix)
			serviceNormalized, _ := NormalizeName(service)
			if len(serviceNormalized) > maxServiceLen {
				serviceNormalized = serviceNormalized[:maxServiceLen]
			}
			name = prefix + serviceNormalized
		}
		// Remove trailing dash if any
		name = strings.TrimSuffix(name, "-")
	}

	return name, nil
}

// NormalizeInfraName normalizes an infra resource name with the raioz prefix
// Format: raioz-{project}-{infra}
func NormalizeInfraName(project, infra string) (string, error) {
	return NormalizeContainerName(project, infra)
}

// NormalizeNetworkName normalizes a network name
// Networks already use project.network, but we can add prefix if needed
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
