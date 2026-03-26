package app

import (
	"encoding/json"
	"fmt"
	"os"

	"raioz/internal/errors"
	"raioz/internal/output"
	"raioz/internal/production"
)

// CompareOptions holds options for the compare use case
type CompareOptions struct {
	ConfigPath     string
	ProductionPath string
	JSONOutput     bool
}

// CompareUseCase handles comparing local config with production
type CompareUseCase struct {
	deps *Dependencies
}

// NewCompareUseCase creates a new CompareUseCase
func NewCompareUseCase(deps *Dependencies) *CompareUseCase {
	return &CompareUseCase{deps: deps}
}

// Execute runs the compare use case
func (uc *CompareUseCase) Execute(opts CompareOptions) error {
	if opts.ProductionPath == "" {
		return errors.New(
			errors.ErrCodeInvalidConfig,
			"Production path required",
		).WithSuggestion(
			"Please specify the path to the production docker-compose.yml file using --production flag",
		)
	}

	// Load local configuration
	deps, warnings, err := uc.deps.ConfigLoader.LoadDeps(opts.ConfigPath)
	if err != nil {
		return errors.New(
			errors.ErrCodeInvalidConfig,
			fmt.Sprintf("Failed to load local config from %s", opts.ConfigPath),
		).WithError(err).WithContext("config_path", opts.ConfigPath)
	}

	// Print warnings
	for _, warning := range warnings {
		output.PrintWarning(warning)
	}

	// Load production configuration
	prodConfig, err := production.LoadComposeFile(opts.ProductionPath)
	if err != nil {
		return errors.New(
			errors.ErrCodeInvalidConfig,
			fmt.Sprintf("Failed to load production config from %s", opts.ProductionPath),
		).WithError(err).WithContext("production_path", opts.ProductionPath)
	}

	// Compare configurations
	result := production.CompareConfigs(deps, prodConfig)

	// Output results
	if opts.JSONOutput {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(result); err != nil {
			return errors.New(
				errors.ErrCodeInvalidConfig,
				"Failed to encode JSON output",
			).WithError(err)
		}
	} else {
		formatted := production.FormatComparisonResult(result)
		fmt.Print(formatted)
	}

	// Exit with error code if there are critical differences
	hasErrors := false
	for _, diff := range result.ServiceDifferences {
		if diff.Severity == "error" || diff.DependsMismatch != nil {
			hasErrors = true
			break
		}
	}

	if hasErrors {
		return errors.New(
			errors.ErrCodeCompatibilityError,
			"Critical differences found between local and production configurations",
		).WithSuggestion(
			"Review the differences above and update your .raioz.json to match production",
		)
	}

	return nil
}
