package validate

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"raioz/internal/config"
	"raioz/internal/docker"
	"raioz/internal/errors"
	"raioz/internal/logging"

	"github.com/xeipuuv/gojsonschema"
)

var profileNameRegex = regexp.MustCompile(`^[a-z0-9-]+$`)

func All(deps *config.Deps) error {
	// JSON Schema validation — only for legacy .raioz.json (schemaVersion "1.0")
	// YAML configs (schemaVersion "2.0") are validated at load time by yaml_loader.go
	if deps.SchemaVersion != "2.0" {
		if err := validateSchema(deps); err != nil {
			return err
		}
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
			"Check service dependencies and version compatibility. " +
				"Review the compatibility warnings for details.",
		).WithError(err)
	}

	// Only fail on errors, not warnings
	if HasCompatibilityErrors(compatIssues) {
		return errors.New(
			errors.ErrCodeCompatibilityError,
			fmt.Sprintf("Compatibility errors detected:\n%s", FormatCompatibilityIssues(compatIssues)),
		).WithSuggestion(
			"Resolve the compatibility issues shown above. " +
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
			"Check your .raioz.json file for invalid data structures. " +
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
			"Schema validation system error. This may indicate a problem with the configuration format. " +
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
			"Fix the validation errors shown above. " +
				"Common issues: missing required fields, incorrect field types, invalid enum values. " +
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
			"Add a 'name' field to the 'project' section in your .raioz.json file. " +
				"Example: {\"project\": {\"name\": \"my-project\", ...}}",
		)
	}
	networkName := deps.Network.GetName()
	if networkName == "" {
		return errors.New(
			errors.ErrCodeMissingField,
			"Project network is required",
		).WithSuggestion(
			"Add a 'network' field to the 'project' section in your .raioz.json file. " +
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
