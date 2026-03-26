package validate

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"raioz/internal/config"
	"raioz/internal/docker"
	"raioz/internal/errors"
)

// ValidateVolumes validates volume configurations before creating
func ValidateVolumes(deps *config.Deps, baseDir string, projectName string) error {
	// Collect all volume names
	volumeNames := make(map[string]bool)
	volumePaths := make(map[string]string) // volume name -> service name

	for name, svc := range deps.Services {
		// Skip if docker is nil (host execution - no docker volumes)
		if svc.Docker == nil {
			continue
		}
		for _, vol := range svc.Docker.Volumes {
			// Parse volume (format: "name:/path" or "/host:/container" or "name")
			parts := strings.Split(vol, ":")
			if len(parts) > 0 && parts[0] != "" {
				volName := parts[0]
				// Check if it's a named volume (not a bind mount)
				if !filepath.IsAbs(volName) {
					volumeNames[volName] = true
					if _, exists := volumePaths[volName]; !exists {
						volumePaths[volName] = name
					}
				}
			}
		}
	}

	// Validate volume names follow Docker naming conventions
	for volName, serviceName := range volumePaths {
		// Docker volume names must be lowercase alphanumeric with hyphens and underscores
		if err := docker.ValidateVolumeName(volName); err != nil {
			return errors.New(
				errors.ErrCodeInvalidField,
				fmt.Sprintf("Service '%s': Invalid volume name '%s'", serviceName, volName),
			).WithSuggestion(
				"Volume names must follow Docker naming conventions. "+
					"Use lowercase letters, numbers, hyphens, and underscores only.",
			).WithContext("service_name", serviceName).WithContext("volume_name", volName).WithError(err)
		}
	}

	return nil
}

// ValidateNetworks validates network configurations before creating
func ValidateNetworks(ctx context.Context, deps *config.Deps) error {
	// Network name is already validated in validateProject
	// Here we can add additional checks like network conflicts with other projects
	// For now, we just ensure the network name is valid
	networkName := deps.Network.GetName()
	if err := docker.ValidateNetworkName(networkName); err != nil {
		return errors.New(
			errors.ErrCodeInvalidField,
			"Invalid network name",
		).WithSuggestion(
			"Network names must follow Docker naming conventions. "+
				"Use lowercase letters, numbers, hyphens, and underscores only.",
		).WithContext("network_name", networkName).WithError(err)
	}

	return nil
}

// ValidateServicePath validates a service path to prevent path traversal
func ValidateServicePath(path string) error {
	// Path must be relative (not absolute)
	if filepath.IsAbs(path) {
		return fmt.Errorf("path must be relative, got absolute path: %s", path)
	}

	// Path must not contain ".." (path traversal)
	if strings.Contains(path, "..") {
		return fmt.Errorf("path must not contain '..': %s", path)
	}

	// Path must not start with "/"
	if strings.HasPrefix(path, "/") {
		return fmt.Errorf("path must be relative, got path starting with '/': %s", path)
	}

	return nil
}

// ValidateImageName validates a Docker image name format
func ValidateImageName(image string) error {
	if image == "" {
		return fmt.Errorf("image name cannot be empty")
	}

	// Basic validation: image name should not contain invalid characters
	// Docker image names can contain: lowercase letters, numbers, hyphens, underscores, dots, slashes
	// But we'll do a basic check here
	if len(image) > 255 {
		return fmt.Errorf("image name too long (max 255 characters)")
	}

	// Check for dangerous characters that could be used for command injection
	dangerousChars := []string{"`", "$", "(", ")", "&", "|", ";", "<", ">", "\n", "\r"}
	for _, char := range dangerousChars {
		if strings.Contains(image, char) {
			return fmt.Errorf("image name contains invalid character: %s", char)
		}
	}

	return nil
}
