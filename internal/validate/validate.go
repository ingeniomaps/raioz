package validate

import (
	"encoding/json"
	"fmt"
	"strings"

	"raioz/internal/config"
	"raioz/internal/docker"
	"raioz/internal/errors"
	"raioz/internal/logging"

	"github.com/xeipuuv/gojsonschema"
)

func All(deps *config.Deps) error {
	// JSON Schema validation
	if err := validateSchema(deps); err != nil {
		// Return the error directly to preserve validation details
		return err
	}

	// Business logic validation
	if err := validateProject(deps); err != nil {
		return err
	}

	if err := validateServices(deps); err != nil {
		return err
	}

	if err := validateInfra(deps); err != nil {
		return err
	}

	if err := validateDependencies(deps); err != nil {
		return err
	}

	// Validate compatibility (warnings only, don't fail validation)
	compatIssues, err := ValidateCompatibility(deps)
	if err != nil {
		return errors.New(
			errors.ErrCodeCompatibilityError,
			"Compatibility validation failed",
		).WithSuggestion(
			"Check service dependencies and version compatibility. "+
				"Review the compatibility warnings for details.",
		).WithError(err)
	}

	// Only fail on errors, not warnings
	if HasCompatibilityErrors(compatIssues) {
		return errors.New(
			errors.ErrCodeCompatibilityError,
			fmt.Sprintf("Compatibility errors detected:\n%s", FormatCompatibilityIssues(compatIssues)),
		).WithSuggestion(
			"Resolve the compatibility issues shown above. "+
				"Check service versions, dependencies, and ensure all required services are defined.",
		)
	}

	// Show warnings if any
	if len(compatIssues) > 0 {
		logging.Warn("Compatibility warnings", "warnings", FormatCompatibilityIssues(compatIssues))
	}

	return nil
}

func validateSchema(deps *config.Deps) error {
	// Convert deps to JSON for schema validation
	data, err := json.Marshal(deps)
	if err != nil {
		return errors.New(
			errors.ErrCodeInvalidConfig,
			"Failed to marshal configuration to JSON",
		).WithSuggestion(
			"Check your .raioz.json file for invalid data structures. "+
				"Ensure all fields have correct types (strings, numbers, booleans, arrays, objects).",
		).WithError(err)
	}

	schemaLoader := gojsonschema.NewStringLoader(config.SchemaJSON)
	documentLoader := gojsonschema.NewBytesLoader(data)

	result, err := gojsonschema.Validate(schemaLoader, documentLoader)
	if err != nil {
		return errors.New(
			errors.ErrCodeSchemaValidation,
			"Failed to validate schema",
		).WithSuggestion(
			"Schema validation system error. This may indicate a problem with the configuration format. "+
				"Try validating your JSON file with a JSON validator.",
		).WithError(err)
	}

	if !result.Valid() {
		var validationErrors []string
		for _, desc := range result.Errors() {
			validationErrors = append(validationErrors, desc.String())
		}
		return errors.New(
			errors.ErrCodeSchemaValidation,
			fmt.Sprintf("Configuration validation errors:\n%s", strings.Join(validationErrors, "\n")),
		).WithSuggestion(
			"Fix the validation errors shown above. "+
				"Common issues: missing required fields, incorrect field types, invalid enum values. "+
				"Refer to the documentation for the correct schema format.",
		)
	}

	return nil
}

func validateProject(deps *config.Deps) error {
	if deps.Project.Name == "" {
		return errors.New(
			errors.ErrCodeMissingField,
			"Project name is required",
		).WithSuggestion(
			"Add a 'name' field to the 'project' section in your .raioz.json file. "+
				"Example: {\"project\": {\"name\": \"my-project\", ...}}",
		)
	}
	networkName := deps.Project.Network.GetName()
	if networkName == "" {
		return errors.New(
			errors.ErrCodeMissingField,
			"Project network is required",
		).WithSuggestion(
			"Add a 'network' field to the 'project' section in your .raioz.json file. "+
				"Example: {\"project\": {\"network\": \"my-network\", ...}}",
		)
	}

	// Validate project name follows naming convention
	if err := docker.ValidateName(deps.Project.Name, docker.MaxContainerNameLength); err != nil {
		return errors.New(
			errors.ErrCodeInvalidField,
			fmt.Sprintf("Project name validation failed: %v", err),
		).WithSuggestion(
			"Project name must be a valid Docker resource name. "+
				"Use lowercase letters, numbers, hyphens, and underscores only. "+
				"Maximum length is 63 characters.",
		).WithContext("project_name", deps.Project.Name).WithError(err)
	}

	// Validate network name follows naming convention
	if err := docker.ValidateNetworkName(networkName); err != nil {
		return errors.New(
			errors.ErrCodeInvalidField,
			fmt.Sprintf("Project network name validation failed: %v", err),
		).WithSuggestion(
			"Network name must be a valid Docker network name. "+
				"Use lowercase letters, numbers, hyphens, and underscores only.",
		).WithContext("network_name", networkName).WithError(err)
	}

	return nil
}

func validateServices(deps *config.Deps) error {
	// Allow 0 services if project.commands is defined or if there's infrastructure
	hasProjectCommands := deps.Project.Commands != nil && (
		deps.Project.Commands.Up != "" ||
		(deps.Project.Commands.Dev != nil && deps.Project.Commands.Dev.Up != "") ||
		(deps.Project.Commands.Prod != nil && deps.Project.Commands.Prod.Up != ""))
	hasInfra := len(deps.Infra) > 0

	if len(deps.Services) == 0 && !hasProjectCommands && !hasInfra {
		return errors.New(
			errors.ErrCodeInvalidConfig,
			"No services, infrastructure, or project commands configured",
		).WithSuggestion(
			"Add at least one service to the 'services' section, configure infrastructure in the 'infra' section, "+
				"or configure 'project.commands.up' in your .raioz.json file. "+
				"Services define the components of your project, infrastructure provides supporting services, "+
				"or you can use project commands to run the project directly.",
		)
	}

	for name, svc := range deps.Services {
		// Validate service name follows naming convention
		if err := docker.ValidateName(name, docker.MaxContainerNameLength); err != nil {
			return errors.New(
				errors.ErrCodeInvalidField,
				fmt.Sprintf("Service '%s': name validation failed", name),
			).WithSuggestion(
				"Service names must be valid Docker resource names. "+
					"Use lowercase letters, numbers, hyphens, and underscores only. "+
					"Maximum length is 63 characters.",
			).WithContext("service_name", name).WithError(err)
		}

		// Validate that container name would be valid
		containerName, err := docker.NormalizeContainerName(deps.Project.Name, name)
		if err != nil {
			return errors.New(
				errors.ErrCodeInvalidField,
				fmt.Sprintf("Service '%s': failed to generate container name", name),
			).WithSuggestion(
				"Container name generation failed. Check that project name and service name are valid. "+
					"The generated name must follow Docker naming conventions.",
			).WithContext("service_name", name).WithContext("project_name", deps.Project.Name).WithError(err)
		}
		if err := docker.ValidateContainerName(containerName); err != nil {
			return errors.New(
				errors.ErrCodeInvalidField,
				fmt.Sprintf("Service '%s': generated container name validation failed", name),
			).WithSuggestion(
				"The generated container name is invalid. "+
					"This may happen if project name + service name combination is too long. "+
					"Consider shortening either the project name or service name.",
			).WithContext("service_name", name).WithContext("container_name", containerName).WithError(err)
		}

		// Validate access field
		if svc.Source.Access != "" && svc.Source.Kind != "git" {
			return errors.New(
				errors.ErrCodeInvalidField,
				fmt.Sprintf("Service '%s': 'access' field can only be used with 'source.kind == \"git\"'", name),
			).WithSuggestion(
				"The 'access' field (readonly/editable) is only valid for Git-based services. "+
					"Remove the 'access' field or change the source kind to 'git'.",
			).WithContext("service_name", name).WithContext("source_kind", svc.Source.Kind)
		}
		if svc.Source.Access != "" && svc.Source.Access != "readonly" && svc.Source.Access != "editable" {
			return errors.New(
				errors.ErrCodeInvalidField,
				fmt.Sprintf("Service '%s': 'access' must be 'readonly' or 'editable', got '%s'", name, svc.Source.Access),
			).WithSuggestion(
				"Set 'access' to either 'readonly' (for read-only Git repositories) or 'editable' (for editable Git repositories). "+
					"Remove the field to use the default (editable).",
			).WithContext("service_name", name).WithContext("access_value", svc.Source.Access)
		}

		// Validate enabled field: if enabled: false, feature flags should not be active
		if svc.Enabled != nil && !*svc.Enabled {
			if svc.FeatureFlag != nil {
				// Check if feature flag would enable the service
				// If feature flag would enable it, that's a conflict
				// We can't check env vars here, so we just warn if feature flag has enabled=true
				if svc.FeatureFlag.Enabled {
					return errors.New(
						errors.ErrCodeInvalidConfig,
						fmt.Sprintf("Service '%s': 'enabled: false' is incompatible with 'featureFlag.enabled: true'", name),
					).WithSuggestion(
						"Remove one of these conflicting settings: either remove 'enabled: false' or set 'featureFlag.enabled' to false. "+
							"A service cannot be both explicitly disabled and enabled via feature flag.",
					).WithContext("service_name", name)
				}
			}
		}

		if svc.Source.Kind == "git" {
			if svc.Source.Repo == "" {
				return errors.New(
					errors.ErrCodeMissingField,
					fmt.Sprintf("Service '%s': git source requires 'repo' field", name),
				).WithSuggestion(
					"Add a 'repo' field to the service's source configuration with the Git repository URL. "+
						"Example: {\"source\": {\"kind\": \"git\", \"repo\": \"https://github.com/user/repo.git\", ...}}",
				).WithContext("service_name", name)
			}
			if svc.Source.Branch == "" {
				return errors.New(
					errors.ErrCodeMissingField,
					fmt.Sprintf("Service '%s': git source requires 'branch' field", name),
				).WithSuggestion(
					"Add a 'branch' field to the service's source configuration with the Git branch name. "+
						"Example: {\"source\": {\"kind\": \"git\", \"branch\": \"main\", ...}}",
				).WithContext("service_name", name)
			}
			if svc.Source.Path == "" {
				return errors.New(
					errors.ErrCodeMissingField,
					fmt.Sprintf("Service '%s': git source requires 'path' field", name),
				).WithSuggestion(
					"Add a 'path' field to the service's source configuration with the path to the service within the repository. "+
						"Example: {\"source\": {\"kind\": \"git\", \"path\": \"./services/my-service\", ...}}",
				).WithContext("service_name", name)
			}

			// If source.command is specified, service runs on host (no docker config needed)
			if svc.Source.Command != "" {
				// Host execution - no docker validation needed
				continue
			}

			// If commands are specified (and no docker), service runs on host (no docker config needed)
			if svc.Commands != nil && svc.Docker == nil {
				// Host execution with commands - no docker validation needed
				continue
			}

			// Dockerfile or command must be specified in docker config
			if svc.Docker == nil {
				return errors.New(
					errors.ErrCodeMissingField,
					fmt.Sprintf("Service '%s': git source requires either 'source.command' or 'commands' for host execution, or 'docker' config for container execution", name),
				).WithSuggestion(
					"For Git-based services, you must specify either: "+
						"1) 'source.command' to run on the host, or "+
						"2) 'commands' (with 'commands.up') to run on the host, or "+
						"3) 'docker' configuration with either 'docker.dockerfile' or 'docker.command'.",
				).WithContext("service_name", name)
			}

			if svc.Docker.Dockerfile == "" && svc.Docker.Command == "" {
				return errors.New(
					errors.ErrCodeMissingField,
					fmt.Sprintf("Service '%s': git source requires either 'dockerfile' or 'command' in docker config", name),
				).WithSuggestion(
					"For Git-based services, you must specify either: "+
						"1) 'docker.dockerfile' with the path to a Dockerfile, or "+
						"2) 'docker.command' with the command to run (requires 'docker.runtime' as well).",
				).WithContext("service_name", name)
			}

			// Validate readonly volumes: readonly services should not have explicit :rw
			// Only validate if docker config exists (not for host execution)
			if svc.Docker != nil && svc.Source.Access == "readonly" {
				for _, vol := range svc.Docker.Volumes {
					if strings.HasSuffix(vol, ":rw") {
						// This is a warning, not an error, because ApplyReadonlyToVolumes will fix it
						// But we should inform the user that :rw is ignored for readonly services
						logging.Warn("Readonly service has explicit :rw volume, will be ignored",
							"service", name,
							"volume", vol)
					}
				}
			}

			// If command is specified, runtime is recommended
			// Only validate if docker config exists (not for host execution)
			if svc.Docker != nil && svc.Docker.Command != "" && svc.Docker.Runtime == "" {
				return errors.New(
					errors.ErrCodeMissingField,
					fmt.Sprintf("Service '%s': 'runtime' is required when 'command' is specified", name),
				).WithSuggestion(
					"Add a 'runtime' field to the docker configuration. "+
						"Valid values: node, go, python, java, rust. "+
						"The runtime is used to generate the Dockerfile wrapper for the command.",
				).WithContext("service_name", name)
			}

			// Validate runtime value if specified
			// Only validate if docker config exists (not for host execution)
			if svc.Docker != nil && svc.Docker.Runtime != "" {
				validRuntimes := map[string]bool{
					"node":   true,
					"go":     true,
					"python": true,
					"java":     true,
					"rust":   true,
				}
				if !validRuntimes[strings.ToLower(svc.Docker.Runtime)] {
					return errors.New(
						errors.ErrCodeInvalidField,
						fmt.Sprintf("Service '%s': invalid runtime '%s'", name, svc.Docker.Runtime),
					).WithSuggestion(
						"Set 'runtime' to one of the supported values: node, go, python, java, rust. "+
							"Use lowercase letters only.",
					).WithContext("service_name", name).WithContext("runtime_value", svc.Docker.Runtime)
				}
			}
		} else if svc.Source.Kind == "image" {
			if svc.Source.Image == "" {
				return errors.New(
					errors.ErrCodeMissingField,
					fmt.Sprintf("Service '%s': image source requires 'image' field", name),
				).WithSuggestion(
					"Add an 'image' field to the service's source configuration with the Docker image name. "+
						"Example: {\"source\": {\"kind\": \"image\", \"image\": \"nginx\", ...}}",
				).WithContext("service_name", name)
			}
			if svc.Source.Tag == "" {
				return errors.New(
					errors.ErrCodeMissingField,
					fmt.Sprintf("Service '%s': image source requires 'tag' field", name),
				).WithSuggestion(
					"Add a 'tag' field to the service's source configuration with the Docker image tag. "+
						"Example: {\"source\": {\"kind\": \"image\", \"tag\": \"latest\", ...}}",
				).WithContext("service_name", name)
			}
		} else if svc.Source.Kind == "local" {
			if svc.Source.Path == "" {
				return errors.New(
					errors.ErrCodeMissingField,
					fmt.Sprintf("Service '%s': local source requires 'path' field", name),
				).WithSuggestion(
					"Add a 'path' field to the service's source configuration with the local path. "+
						"Example: {\"source\": {\"kind\": \"local\", \"path\": \".\", ...}}",
				).WithContext("service_name", name)
			}
			// If source.command is specified, service runs on host (no docker config needed)
			if svc.Source.Command != "" {
				// Host execution - no docker validation needed
				continue
			}
			// If commands are specified (and no docker), service runs on host (no docker config needed)
			if svc.Commands != nil && svc.Docker == nil {
				// Host execution with commands - no docker validation needed
				continue
			}
		} else {
			return errors.New(
				errors.ErrCodeInvalidField,
				fmt.Sprintf("Service '%s': invalid source kind '%s'", name, svc.Source.Kind),
			).WithSuggestion(
				"Set 'source.kind' to either 'git' (for Git-based services), 'image' (for Docker image-based services), or 'local' (for local path-based services).",
			).WithContext("service_name", name).WithContext("source_kind", svc.Source.Kind)
		}

		// Validate profiles
		for _, profile := range svc.Profiles {
			if profile != "frontend" && profile != "backend" {
				return errors.New(
					errors.ErrCodeInvalidField,
					fmt.Sprintf("Service '%s': invalid profile '%s'", name, profile),
				).WithSuggestion(
					"Set 'profile' to either 'frontend' or 'backend'. "+
						"Profiles are used to categorize services and can be used for filtering.",
				).WithContext("service_name", name).WithContext("profile_value", profile)
			}
		}

		// Feature flags and mocks are validated separately in config.ValidateFeatureFlags
	}

	return nil
}

func validateInfra(deps *config.Deps) error {
	// Validate each infra service
	for name, infra := range deps.Infra {
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
	}
	for name, infra := range deps.Infra {
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

		// Validate that container name would be valid
		containerName, err := docker.NormalizeInfraName(deps.Project.Name, name)
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

		if infra.Image == "" {
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

	// Validate dependsOn references
	for name, svc := range deps.Services {
		// Skip if docker is nil (host execution)
		if svc.Docker == nil {
			continue
		}
		for _, dep := range svc.Docker.DependsOn {
			if !allNames[dep] {
				return errors.New(
					errors.ErrCodeInvalidField,
					fmt.Sprintf("Service '%s': depends on '%s' which does not exist", name, dep),
				).WithSuggestion(
					fmt.Sprintf("Either add a service or infrastructure named '%s', or remove it from the 'dependsOn' list of service '%s'. "+
						"Dependencies must reference existing services or infrastructure components.", dep, name),
				).WithContext("service_name", name).WithContext("missing_dependency", dep)
			}
		}
	}

	// Validate dependency cycles (using docker package function)
	if err := docker.ValidateDependencyCycle(deps); err != nil {
		return err
	}

	return nil
}
