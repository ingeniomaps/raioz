package app

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"raioz/internal/audit"
	"raioz/internal/errors"
	"raioz/internal/i18n"
	"raioz/internal/override"
)

// OverrideUseCase handles override operations for services
type OverrideUseCase struct {
	deps *Dependencies
	Out  io.Writer
}

// NewOverrideUseCase creates a new OverrideUseCase
func NewOverrideUseCase(deps *Dependencies) *OverrideUseCase {
	return &OverrideUseCase{deps: deps, Out: os.Stdout}
}

// Apply applies an override for a service
func (uc *OverrideUseCase) Apply(serviceName string, overridePath string, configPath string) error {
	if overridePath == "" {
		return errors.New(errors.ErrCodeInvalidField, i18n.T("error.override_path_required"))
	}

	w := uc.Out

	absPath, err := filepath.Abs(overridePath)
	if err != nil {
		return errors.New(errors.ErrCodeInvalidField, i18n.T("error.override_resolve_path")).WithError(err)
	}

	if err := override.ValidateOverridePath(absPath); err != nil {
		return errors.New(errors.ErrCodeInvalidField, i18n.T("error.override_invalid_path")).WithError(err)
	}

	var projectName string
	if configPath != "" {
		deps, _, _ := uc.deps.ConfigLoader.LoadDeps(configPath)
		if deps != nil {
			projectName = deps.Project.Name
			if _, ok := deps.Services[serviceName]; !ok {
				fmt.Fprintf(w, "⚠️  %s\n", i18n.T("output.override_service_not_found", serviceName))
			}
		}
	}

	hasExisting, err := override.HasOverride(serviceName)
	if err != nil {
		return errors.New(errors.ErrCodeWorkspaceError, i18n.T("error.override_check")).WithError(err)
	}

	if hasExisting {
		existing, _ := override.GetOverride(serviceName)
		if existing != nil {
			fmt.Fprintf(w, "⚠️  %s\n", i18n.T("output.override_already_exists", serviceName, existing.Path))
			fmt.Fprintf(w, "ℹ️  %s\n", i18n.T("output.override_replacing"))
		}
	}

	overrideConfig := override.Override{
		Path:   absPath,
		Mode:   "local",
		Source: "external",
	}

	if err := override.AddOverride(serviceName, overrideConfig); err != nil {
		return errors.New(errors.ErrCodeWorkspaceError, i18n.T("error.override_add")).WithError(err)
	}

	if err := audit.LogOverrideApplied(serviceName, absPath); err != nil {
		fmt.Fprintf(w, "⚠️  %s\n", i18n.T("output.failed_log_audit", err))
	}

	fmt.Fprintf(w, "✔ %s\n", i18n.T("output.override_registered", serviceName, absPath))

	if projectName != "" {
		fmt.Fprintf(w, "ℹ️  %s\n", i18n.T("output.override_run_up_project", projectName))
	} else {
		fmt.Fprintf(w, "ℹ️  %s\n", i18n.T("output.override_run_up"))
	}

	return nil
}

// List lists all service overrides
func (uc *OverrideUseCase) List() error {
	w := uc.Out

	overrides, err := override.LoadOverrides()
	if err != nil {
		return errors.New(errors.ErrCodeWorkspaceError, i18n.T("error.override_load")).WithError(err)
	}

	if len(overrides) == 0 {
		fmt.Fprintln(w, i18n.T("output.override_empty_list"))
		return nil
	}

	fmt.Fprintln(w, i18n.T("output.override_list_header"))
	fmt.Fprintln(w)
	for serviceName, overrideConfig := range overrides {
		pathStatus := "✓"
		if err := override.ValidateOverridePath(overrideConfig.Path); err != nil {
			pathStatus = "✗ (path does not exist)"
		}

		fmt.Fprintf(w, "  %s:\n", serviceName)
		fmt.Fprintf(w, "    Path:   %s %s\n", overrideConfig.Path, pathStatus)
		fmt.Fprintf(w, "    Mode:   %s\n", overrideConfig.Mode)
		fmt.Fprintf(w, "    Source: %s\n", overrideConfig.Source)
		fmt.Fprintln(w)
	}

	removed, err := override.CleanInvalidOverrides()
	if err != nil {
		fmt.Fprintf(w, "⚠️  %s\n", i18n.T("output.failed_clean_overrides", err))
	} else if len(removed) > 0 {
		fmt.Fprintf(w, "⚠️  %s\n", i18n.T("output.override_removed_invalid", len(removed), removed))
	}

	return nil
}

// Remove removes a service override
func (uc *OverrideUseCase) Remove(serviceName string) error {
	w := uc.Out

	hasOverride, err := override.HasOverride(serviceName)
	if err != nil {
		return errors.New(errors.ErrCodeWorkspaceError, i18n.T("error.override_check")).WithError(err)
	}

	if !hasOverride {
		return errors.New(errors.ErrCodeInvalidField, i18n.T("error.override_not_found", serviceName))
	}

	if err := override.RemoveOverride(serviceName); err != nil {
		return errors.New(errors.ErrCodeWorkspaceError, i18n.T("error.override_remove")).WithError(err)
	}

	if err := audit.LogOverrideReverted(serviceName, "user removed override"); err != nil {
		fmt.Fprintf(w, "⚠️  %s\n", i18n.T("output.failed_log_audit", err))
	}

	fmt.Fprintf(w, "✔ %s\n", i18n.T("output.override_removed", serviceName))
	fmt.Fprintf(w, "ℹ️  %s\n", i18n.T("output.override_run_up_apply"))

	return nil
}
