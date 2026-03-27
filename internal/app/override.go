package app

import (
	"fmt"
	"path/filepath"

	"raioz/internal/audit"
	"raioz/internal/i18n"
	"raioz/internal/output"
	"raioz/internal/override"
)

// OverrideUseCase handles override operations for services
type OverrideUseCase struct {
	deps *Dependencies
}

// NewOverrideUseCase creates a new OverrideUseCase
func NewOverrideUseCase(deps *Dependencies) *OverrideUseCase {
	return &OverrideUseCase{deps: deps}
}

// Apply applies an override for a service
func (uc *OverrideUseCase) Apply(serviceName string, overridePath string, configPath string) error {
	if overridePath == "" {
		return fmt.Errorf("--path is required")
	}

	// Validate path
	absPath, err := filepath.Abs(overridePath)
	if err != nil {
		return fmt.Errorf("failed to resolve override path: %w", err)
	}

	if err := override.ValidateOverridePath(absPath); err != nil {
		return fmt.Errorf("invalid override path: %w", err)
	}

	// Check if service exists in current config (optional check)
	var projectName string
	if configPath != "" {
		deps, _, _ := uc.deps.ConfigLoader.LoadDeps(configPath)
		if deps != nil {
			projectName = deps.Project.Name
			if _, ok := deps.Services[serviceName]; !ok {
				output.PrintWarning(i18n.T("output.override_service_not_found", serviceName))
			}
		}
	}

	// Check if override already exists
	hasOverride, err := override.HasOverride(serviceName)
	if err != nil {
		return fmt.Errorf("failed to check existing override: %w", err)
	}

	if hasOverride {
		existingOverride, err := override.GetOverride(serviceName)
		if err != nil {
			return fmt.Errorf("failed to get existing override: %w", err)
		}
		output.PrintWarning(i18n.T("output.override_already_exists", serviceName, existingOverride.Path))
		output.PrintInfo(i18n.T("output.override_replacing"))
	}

	// Add override
	overrideConfig := override.Override{
		Path:   absPath,
		Mode:   "local",
		Source: "external",
	}

	if err := override.AddOverride(serviceName, overrideConfig); err != nil {
		return fmt.Errorf("failed to add override: %w", err)
	}

	// Log audit event
	if err := audit.LogOverrideApplied(serviceName, absPath); err != nil {
		output.PrintWarning(i18n.T("output.failed_log_audit", err))
	}

	output.PrintSuccess(i18n.T("output.override_registered", serviceName, absPath))

	if projectName != "" {
		output.PrintInfo(i18n.T("output.override_run_up_project", projectName))
	} else {
		output.PrintInfo(i18n.T("output.override_run_up"))
	}

	return nil
}

// List lists all service overrides
func (uc *OverrideUseCase) List() error {
	overrides, err := override.LoadOverrides()
	if err != nil {
		return fmt.Errorf("failed to load overrides: %w", err)
	}

	if len(overrides) == 0 {
		fmt.Println(i18n.T("output.override_empty_list"))
		return nil
	}

	fmt.Println(i18n.T("output.override_list_header"))
	fmt.Println()
	for serviceName, overrideConfig := range overrides {
		pathStatus := "✓"
		if err := override.ValidateOverridePath(overrideConfig.Path); err != nil {
			pathStatus = "✗ (path does not exist)"
		}

		fmt.Printf("  %s:\n", serviceName)
		fmt.Printf("    Path:   %s %s\n", overrideConfig.Path, pathStatus)
		fmt.Printf("    Mode:   %s\n", overrideConfig.Mode)
		fmt.Printf("    Source: %s\n", overrideConfig.Source)
		fmt.Println()
	}

	// Clean invalid overrides
	removed, err := override.CleanInvalidOverrides()
	if err != nil {
		output.PrintWarning(i18n.T("output.failed_clean_overrides", err))
	} else if len(removed) > 0 {
		fmt.Printf("⚠️  Removed %d invalid override(s): %v\n", len(removed), removed)
	}

	return nil
}

// Remove removes a service override
func (uc *OverrideUseCase) Remove(serviceName string) error {
	hasOverride, err := override.HasOverride(serviceName)
	if err != nil {
		return fmt.Errorf("failed to check override: %w", err)
	}

	if !hasOverride {
		return fmt.Errorf("no override found for service '%s'", serviceName)
	}

	if err := override.RemoveOverride(serviceName); err != nil {
		return fmt.Errorf("failed to remove override: %w", err)
	}

	// Log audit event
	if err := audit.LogOverrideReverted(serviceName, "user removed override"); err != nil {
		output.PrintWarning(i18n.T("output.failed_log_audit", err))
	}

	output.PrintSuccess(i18n.T("output.override_removed", serviceName))
	output.PrintInfo(i18n.T("output.override_run_up_apply"))

	return nil
}
