package validate

import (
	"fmt"

	"raioz/internal/config"
	"raioz/internal/docker"
	"raioz/internal/errors"
)

func validateInfra(deps *config.Deps) error {
	// Validate each infra service
	for name, entry := range deps.Infra {
		if entry.Inline == nil {
			// Path-based: validate file exists when we have project dir (optional); skip here
			continue
		}
		infra := *entry.Inline
		// Check if image is empty (schema validation should catch this, but provide clearer message)
		if infra.Image == "" {
			return errors.New(
				errors.ErrCodeMissingField,
				fmt.Sprintf("Infra '%s': 'image' field is required and cannot be empty", name),
			).WithSuggestion(
				fmt.Sprintf("Add a valid 'image' field to infra '%s'. "+
					"Example: {\"infra\": {\"%s\": {\"image\": \"postgres\", \"tag\": \"15\"}}}", name, name),
			).WithContext("infra_name", name)
		}
		// Validate profiles (lowercase letters, digits, hyphens only)
		for _, p := range entry.Inline.Profiles {
			if !profileNameRegex.MatchString(p) {
				return errors.New(
					errors.ErrCodeInvalidField,
					fmt.Sprintf("Infra '%s': invalid profile '%s'", name, p),
				).WithSuggestion(
					"Profile names must be lowercase letters, digits and hyphens only (e.g. frontend, backend, load-balancer).",
				).WithContext("infra_name", name).WithContext("profile_value", p)
			}
		}
	}
	for name, entry := range deps.Infra {
		// Validate infra name follows naming convention
		if err := docker.ValidateName(name, docker.MaxContainerNameLength); err != nil {
			return errors.New(
				errors.ErrCodeInvalidField,
				fmt.Sprintf("Infrastructure '%s': name validation failed", name),
			).WithSuggestion(
				"Infrastructure names must be valid Docker resource names. "+
					"Use lowercase letters, numbers, hyphens, and underscores only. "+
					"Maximum length is 63 characters.",
			).WithContext("infra_name", name).WithError(err)
		}

		if entry.Inline == nil {
			// Path-based entry: no further validation here (file existence checked at compose time)
			continue
		}

		// Validate that container name would be valid (inline only)
		workspaceName := deps.GetWorkspaceName()
		hasExplicitWorkspace := deps.HasExplicitWorkspace()
		containerName, err := docker.NormalizeInfraName(workspaceName, name, deps.Project.Name, hasExplicitWorkspace)
		if err != nil {
			return errors.New(
				errors.ErrCodeInvalidField,
				fmt.Sprintf("Infrastructure '%s': failed to generate container name", name),
			).WithSuggestion(
				"Container name generation failed. Check that project name and infrastructure name are valid. "+
					"The generated name must follow Docker naming conventions.",
			).WithContext("infra_name", name).WithContext("project_name", deps.Project.Name).WithError(err)
		}
		if err := docker.ValidateContainerName(containerName); err != nil {
			return errors.New(
				errors.ErrCodeInvalidField,
				fmt.Sprintf("Infrastructure '%s': generated container name validation failed", name),
			).WithSuggestion(
				"The generated container name is invalid. "+
					"This may happen if project name + infrastructure name combination is too long. "+
					"Consider shortening either the project name or infrastructure name.",
			).WithContext("infra_name", name).WithContext("container_name", containerName).WithError(err)
		}

		if entry.Inline.Image == "" {
			return errors.New(
				errors.ErrCodeMissingField,
				fmt.Sprintf("Infrastructure '%s': 'image' field is required", name),
			).WithSuggestion(
				"Add an 'image' field to the infrastructure configuration with the Docker image name. "+
					"Example: {\"infra\": {\"my-db\": {\"image\": \"postgres:15\", ...}}}",
			).WithContext("infra_name", name)
		}
	}
	return nil
}

func validateDependencies(deps *config.Deps) error {
	// Collect all service and infra names
	allNames := make(map[string]bool)
	for name := range deps.Services {
		allNames[name] = true
	}
	for name := range deps.Infra {
		allNames[name] = true
	}

	// Validate dependsOn references (service-level and docker-level)
	for name, svc := range deps.Services {
		for _, dep := range svc.GetDependsOn() {
			if !allNames[dep] {
				return errors.New(
					errors.ErrCodeInvalidField,
					fmt.Sprintf("Service '%s': depends on '%s' which does not exist", name, dep),
				).WithSuggestion(
					fmt.Sprintf(
						"Either add a service or infrastructure "+
							"named '%s', or remove it from the "+
							"'dependsOn' list of service '%s'. "+
							"Dependencies must reference existing "+
							"services or infrastructure components.",
						dep, name,
					),
				).WithContext("service_name", name).
					WithContext("missing_dependency", dep)
			}
		}
	}

	// Validate dependency cycles (using docker package function)
	if err := docker.ValidateDependencyCycle(deps); err != nil {
		return err
	}

	return nil
}
