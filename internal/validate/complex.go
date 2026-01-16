package validate

import (
	"fmt"
	"path/filepath"
	"strings"

	"raioz/internal/config"
	"raioz/internal/docker"
	"raioz/internal/errors"
)

// ValidateComplexConfiguration performs additional validations for complex configurations
// This is called after basic validation to catch complex issues
func ValidateComplexConfiguration(deps *config.Deps) error {
	// Validate circular dependencies (already done in validateDependencies, but double-check)
	if err := docker.ValidateDependencyCycle(deps); err != nil {
		return err
	}

	// Validate that all dependsOn references are valid (already done, but ensure)
	if err := validateDependencies(deps); err != nil {
		return err
	}

	// Validate profile consistency
	if err := ValidateProfileConsistency(deps); err != nil {
		return err
	}

	// Validate environment variable references
	if err := ValidateEnvReferences(deps); err != nil {
		return err
	}

	// Validate volume mount paths
	if err := ValidateVolumeMounts(deps); err != nil {
		return err
	}

	return nil
}

// ValidateProfileConsistency validates that profiles are used consistently
func ValidateProfileConsistency(deps *config.Deps) error {
	// Check that services with profiles don't have conflicting configurations
	profileServices := make(map[string][]string) // profile -> service names

	for name, svc := range deps.Services {
		for _, profile := range svc.Profiles {
			profileServices[profile] = append(profileServices[profile], name)
		}
	}

	// Validate that profile names are valid
	validProfiles := map[string]bool{
		"frontend": true,
		"backend":  true,
	}

	for profile, services := range profileServices {
		if !validProfiles[profile] {
			return errors.New(
				errors.ErrCodeInvalidField,
				fmt.Sprintf("Invalid profile '%s' used by services: %v", profile, services),
			).WithSuggestion(
				"Valid profiles are: 'frontend', 'backend'. "+
					"Remove invalid profiles or use valid profile names.",
			).WithContext("profile", profile).WithContext("services", services)
		}
	}

	return nil
}

// ValidateEnvReferences validates environment variable references
func ValidateEnvReferences(deps *config.Deps) error {
	// Check that env files referenced in configuration exist
	// This is a preventive check to avoid runtime errors
	for _, envFile := range deps.Env.Files {
		if envFile == "" {
			continue
		}

		// Env files are relative to the env directory, so we can't validate existence here
		// But we can validate the format
		if filepath.IsAbs(envFile) {
			return errors.New(
				errors.ErrCodeInvalidField,
				fmt.Sprintf("Environment file path must be relative: %s", envFile),
			).WithSuggestion(
				"Environment file paths must be relative to the env directory. "+
					"Use relative paths like 'global.env' or 'services/my-service.env'.",
			).WithContext("env_file", envFile)
		}

		// Prevent path traversal
		if strings.Contains(envFile, "..") {
			return errors.New(
				errors.ErrCodeInvalidField,
				fmt.Sprintf("Environment file path must not contain '..': %s", envFile),
			).WithSuggestion(
				"Environment file paths must not contain '..'. "+
					"Use relative paths within the env directory.",
			).WithContext("env_file", envFile)
		}
	}

	return nil
}

// ValidateVolumeMounts validates volume mount configurations
func ValidateVolumeMounts(deps *config.Deps) error {
	for name, svc := range deps.Services {
		// Skip if docker is nil (host execution - no docker volumes)
		if svc.Docker == nil {
			continue
		}
		for _, vol := range svc.Docker.Volumes {
			// Parse volume (format: "source:destination[:options]")
			parts := strings.Split(vol, ":")
			if len(parts) < 2 {
				continue // Skip invalid formats (will be caught by other validations)
			}

			source := parts[0]
			destination := parts[1]

			// Validate destination path (must be absolute in container)
			if !filepath.IsAbs(destination) {
				return errors.New(
					errors.ErrCodeInvalidField,
					fmt.Sprintf("Service '%s': Volume destination must be absolute path: %s", name, destination),
				).WithSuggestion(
					"Volume mount destinations must be absolute paths in the container. "+
						"Example: '/app/data' not 'app/data'.",
				).WithContext("service_name", name).WithContext("volume", vol)
			}

			// If source is a bind mount (absolute path), validate it
			if filepath.IsAbs(source) {
				// Prevent path traversal in bind mounts
				if strings.Contains(source, "..") {
					return errors.New(
						errors.ErrCodeInvalidField,
						fmt.Sprintf("Service '%s': Volume source path must not contain '..': %s", name, source),
					).WithSuggestion(
						"Volume source paths must not contain '..'. "+
							"Use absolute paths without '..' components.",
					).WithContext("service_name", name).WithContext("volume", vol)
				}
			}
		}
	}

	return nil
}
