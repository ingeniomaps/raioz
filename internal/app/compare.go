package app

import (
	"encoding/json"
	"fmt"
	"os"

	"raioz/internal/errors"
	"raioz/internal/i18n"
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
			i18n.T("error.production_path_required"),
		).WithSuggestion(
			i18n.T("error.production_path_required_suggestion"),
		)
	}

	// Load local configuration
	deps, warnings, err := uc.deps.ConfigLoader.LoadDeps(opts.ConfigPath)
	if err != nil {
		return errors.New(
			errors.ErrCodeInvalidConfig,
			i18n.T("error.config_load_local", opts.ConfigPath),
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
			i18n.T("error.config_load_production", opts.ProductionPath),
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
				i18n.T("error.json_encode"),
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
			i18n.T("error.compare_critical"),
		).WithSuggestion(
			i18n.T("error.compare_critical_suggestion"),
		)
	}

	return nil
}
