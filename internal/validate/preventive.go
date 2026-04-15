package validate

import (
	"context"
	"fmt"
	"os"
	"strings"

	"raioz/internal/config"
	"raioz/internal/docker"
	"raioz/internal/errors"
	"raioz/internal/workspace"
)

// ValidateBeforeUp performs all validations needed before running 'raioz up'
// This includes preflight checks, configuration validation, and preventive checks
func ValidateBeforeUp(ctx context.Context, deps *config.Deps, ws *workspace.Workspace) error {
	// Step 1: Preflight checks (Docker, Git, disk space, network)
	if err := PreflightCheckWithContext(ctx); err != nil {
		return errors.New(
			errors.ErrCodeDockerNotInstalled,
			"Preflight checks failed",
		).WithSuggestion(
			"Ensure Docker is installed and running, Git is installed, and you have sufficient disk space. " +
				"Check the error details above for specific issues.",
		).WithError(err)
	}

	// Step 2: Workspace permissions
	if err := CheckWorkspacePermissions(ws.Root); err != nil {
		return err
	}

	// Step 3: Full configuration validation
	if err := All(deps); err != nil {
		return err
	}

	// Step 3.5: Complex configuration validation
	if err := ValidateComplexConfiguration(deps); err != nil {
		return err
	}

	// Step 4: Validate feature flags
	if err := config.ValidateFeatureFlags(deps); err != nil {
		return errors.New(
			errors.ErrCodeInvalidConfig,
			"Feature flag validation failed",
		).WithSuggestion(
			"Check your feature flag configurations. " +
				"Ensure environment variables are set correctly and feature flags reference valid services.",
		).WithError(err)
	}

	// Step 5: Validate ports (preventive check before starting services)
	baseDir := workspace.GetBaseDirFromWorkspace(ws)
	conflicts, err := docker.ValidatePorts(deps, baseDir, deps.Project.Name)
	if err != nil {
		return errors.New(
			errors.ErrCodePortConflict,
			"Port validation failed",
		).WithSuggestion(
			"Check that ports are not already in use by other services. " +
				"Use 'raioz ports' to see active ports.",
		).WithError(err)
	}
	if len(conflicts) > 0 {
		conflictMsg := "Port conflicts detected:\n"
		for _, conflict := range conflicts {
			conflictMsg += fmt.Sprintf(
				"  Port %s is used by project '%s', service '%s'",
				conflict.Port, conflict.Project, conflict.Service,
			)
			if conflict.Alternative != "" {
				conflictMsg += fmt.Sprintf(" (suggested alternative: %s)", conflict.Alternative)
			}
			conflictMsg += "\n"
		}
		return errors.New(
			errors.ErrCodePortConflict,
			conflictMsg,
		).WithSuggestion(
			"Resolve port conflicts by changing port mappings in your configuration. " +
				"Each service must use unique host ports.",
		)
	}

	// Step 6: Validate Git repositories (preventive check before cloning)
	if err := ValidateGitRepositories(ctx, deps); err != nil {
		return err
	}

	// Step 7: Validate Docker images (preventive check before pulling)
	if err := ValidateDockerImages(ctx, deps); err != nil {
		return err
	}

	// Step 8: Validate volumes (preventive check before creating)
	if err := ValidateVolumes(deps, baseDir, deps.Project.Name); err != nil {
		return err
	}

	// Step 9: Validate networks (preventive check before creating)
	return ValidateNetworks(ctx, deps)
}

// ValidateBeforeDown performs validations needed before running 'raioz down'
func ValidateBeforeDown(ctx context.Context, ws *workspace.Workspace) error {
	// Step 1: Preflight checks (Docker must be running)
	if err := PreflightCheckWithContext(ctx); err != nil {
		return errors.New(
			errors.ErrCodeDockerNotRunning,
			"Preflight checks failed",
		).WithSuggestion(
			"Docker must be running to stop services. " +
				"Start Docker daemon and try again.",
		).WithError(err)
	}

	// Step 2: Workspace must exist
	if _, err := os.Stat(ws.Root); os.IsNotExist(err) {
		return errors.New(
			errors.ErrCodeWorkspaceError,
			"Workspace does not exist",
		).WithSuggestion(
			"The project may not have been started yet. "+
				"Run 'raioz up' to start the project.",
		).WithContext("workspace", ws.Root)
	}

	return nil
}

// ValidateGitRepositories validates Git repository configurations before cloning
func ValidateGitRepositories(ctx context.Context, deps *config.Deps) error {
	for name, svc := range deps.Services {
		if svc.Source.Kind == "git" {
			// Validate repository URL format
			if svc.Source.Repo == "" {
				return errors.New(
					errors.ErrCodeMissingField,
					fmt.Sprintf("Service '%s': Git repository URL is required", name),
				).WithSuggestion(
					"Add a 'repo' field to the service's source configuration with a valid Git repository URL.",
				).WithContext("service_name", name)
			}

			// Validate branch name (basic validation - no dangerous characters)
			if svc.Source.Branch == "" {
				return errors.New(
					errors.ErrCodeMissingField,
					fmt.Sprintf("Service '%s': Git branch is required", name),
				).WithSuggestion(
					"Add a 'branch' field to the service's source configuration with a valid branch name.",
				).WithContext("service_name", name)
			}

			// Validate branch name for command injection prevention
			// Use internal validation function (validateBranch is not exported, so we validate manually)
			if svc.Source.Branch == "" {
				return errors.New(
					errors.ErrCodeMissingField,
					fmt.Sprintf("Service '%s': Git branch is required", name),
				).WithSuggestion(
					"Add a 'branch' field to the service's source configuration.",
				).WithContext("service_name", name)
			}
			// Basic validation: check for dangerous characters
			dangerousChars := []string{";", "|", "&", "$", "`", "\n", "\r", "\t"}
			for _, char := range dangerousChars {
				if strings.Contains(svc.Source.Branch, char) {
					return errors.New(
						errors.ErrCodeInvalidField,
						fmt.Sprintf("Service '%s': Branch name contains dangerous character", name),
					).WithSuggestion(
						"Branch names must not contain dangerous characters. "+
							"Use alphanumeric characters, hyphens, slashes, underscores, and dots only.",
					).WithContext("service_name", name).WithContext("branch", svc.Source.Branch)
				}
			}

			// Validate repository URL for command injection prevention
			if svc.Source.Repo == "" {
				return errors.New(
					errors.ErrCodeMissingField,
					fmt.Sprintf("Service '%s': Git repository URL is required", name),
				).WithSuggestion(
					"Add a 'repo' field to the service's source configuration.",
				).WithContext("service_name", name)
			}
			// Basic validation: check for dangerous characters and valid format
			for _, char := range dangerousChars {
				if strings.Contains(svc.Source.Repo, char) {
					return errors.New(
						errors.ErrCodeInvalidField,
						fmt.Sprintf("Service '%s': Repository URL contains dangerous character", name),
					).WithSuggestion(
						"Repository URLs must not contain dangerous characters. "+
							"Use valid Git repository URLs (HTTPS or SSH format).",
					).WithContext("service_name", name).WithContext("repo", svc.Source.Repo)
				}
			}
			// Validate URL format
			validPrefixes := []string{"ssh://", "https://", "http://", "git@", "file://"}
			hasValidPrefix := false
			for _, prefix := range validPrefixes {
				if strings.HasPrefix(svc.Source.Repo, prefix) {
					hasValidPrefix = true
					break
				}
			}
			if !hasValidPrefix {
				return errors.New(
					errors.ErrCodeInvalidField,
					fmt.Sprintf("Service '%s': Invalid repository URL format", name),
				).WithSuggestion(
					"Repository URLs must start with ssh://, https://, http://, git@, or file://.",
				).WithContext("service_name", name).WithContext("repo", svc.Source.Repo)
			}

			// Validate path (prevent path traversal)
			if svc.Source.Path != "" {
				if err := ValidateServicePath(svc.Source.Path); err != nil {
					return errors.New(
						errors.ErrCodeInvalidField,
						fmt.Sprintf("Service '%s': Invalid path", name),
					).WithSuggestion(
						"Service paths must be relative paths within the repository. "+
							"Avoid '..' and absolute paths.",
					).WithContext("service_name", name).WithContext("path", svc.Source.Path).WithError(err)
				}
			}
		}
	}

	return nil
}

// ValidateDockerImages validates Docker image configurations before pulling
func ValidateDockerImages(ctx context.Context, deps *config.Deps) error {
	// Validate service images
	for name, svc := range deps.Services {
		if svc.Source.Kind == "image" {
			if svc.Source.Image == "" {
				return errors.New(
					errors.ErrCodeMissingField,
					fmt.Sprintf("Service '%s': Docker image name is required", name),
				).WithSuggestion(
					"Add an 'image' field to the service's source configuration.",
				).WithContext("service_name", name)
			}
			if svc.Source.Tag == "" {
				return errors.New(
					errors.ErrCodeMissingField,
					fmt.Sprintf("Service '%s': Docker image tag is required", name),
				).WithSuggestion(
					"Add a 'tag' field to the service's source configuration.",
				).WithContext("service_name", name)
			}

			// Validate image name format (basic validation)
			if err := ValidateImageName(svc.Source.Image); err != nil {
				return errors.New(
					errors.ErrCodeInvalidField,
					fmt.Sprintf("Service '%s': Invalid image name", name),
				).WithSuggestion(
					"Docker image names must follow Docker naming conventions. "+
						"Format: [registry/]repository[:tag] or [registry/]namespace/repository[:tag]",
				).WithContext("service_name", name).WithContext("image", svc.Source.Image).WithError(err)
			}
		}
	}

	// Validate inline infra images (path-based YAML is not validated here)
	for name, entry := range deps.Infra {
		if entry.Inline == nil {
			continue
		}
		infra := *entry.Inline
		// Compose-mode deps supply their own image inside the user's
		// compose fragment; no image validation here. Skip the image
		// checks entirely in that mode.
		if len(infra.Compose) > 0 {
			continue
		}
		if infra.Image == "" {
			return errors.New(
				errors.ErrCodeMissingField,
				fmt.Sprintf("Infrastructure '%s': Docker image name is required", name),
			).WithSuggestion(
				"Add an 'image' field to the infrastructure configuration.",
			).WithContext("infra_name", name)
		}

		if err := ValidateImageName(infra.Image); err != nil {
			return errors.New(
				errors.ErrCodeInvalidField,
				fmt.Sprintf("Infrastructure '%s': Invalid image name", name),
			).WithSuggestion(
				"Docker image names must follow Docker naming conventions.",
			).WithContext("infra_name", name).WithContext("image", infra.Image).WithError(err)
		}
	}

	return nil
}
