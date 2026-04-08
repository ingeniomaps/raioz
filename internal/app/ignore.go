package app

import (
	"fmt"
	"io"
	"os"

	"raioz/internal/config"
	"raioz/internal/errors"
	"raioz/internal/i18n"
	"raioz/internal/ignore"
)

// IgnoreUseCase handles ignore operations for services
type IgnoreUseCase struct {
	deps *Dependencies
	Out  io.Writer
}

// NewIgnoreUseCase creates a new IgnoreUseCase
func NewIgnoreUseCase(deps *Dependencies) *IgnoreUseCase {
	return &IgnoreUseCase{deps: deps, Out: os.Stdout}
}

// Add adds a service to the ignore list
func (uc *IgnoreUseCase) Add(serviceName string, configPath string) error {
	if serviceName == "" {
		return errors.New(errors.ErrCodeInvalidField, i18n.T("error.ignore_name_empty"))
	}

	w := uc.Out

	isIgnored, err := ignore.IsIgnored(serviceName)
	if err != nil {
		return errors.New(errors.ErrCodeWorkspaceError, i18n.T("error.ignore_check")).WithError(err)
	}

	if isIgnored {
		fmt.Fprintf(w, "ℹ️  %s\n", i18n.T("output.ignore_already_ignored", serviceName))
		return nil
	}

	if err := ignore.AddService(serviceName); err != nil {
		return errors.New(errors.ErrCodeWorkspaceError, i18n.T("error.ignore_add")).WithError(err)
	}

	fmt.Fprintf(w, "✔ %s\n", i18n.T("output.ignore_added", serviceName))
	fmt.Fprintf(w, "ℹ️  %s\n", i18n.T("output.ignore_next_up"))

	deps, _, _ := uc.deps.ConfigLoader.LoadDeps(configPath)
	if deps != nil {
		if _, exists := deps.Services[serviceName]; exists {
			dependents := findDependents(deps, serviceName)
			if len(dependents) > 0 {
				fmt.Fprintf(w, "⚠️  %s\n",
					i18n.T("output.ignore_dependents_warning", serviceName, dependents))
			}
		}
	}

	return nil
}

// findDependents returns services that depend on the given service
func findDependents(deps *config.Deps, serviceName string) []string {
	var dependents []string
	for name, svc := range deps.Services {
		for _, dep := range svc.GetDependsOn() {
			if dep == serviceName {
				dependents = append(dependents, name)
				break
			}
		}
	}
	return dependents
}

// Remove removes a service from the ignore list
func (uc *IgnoreUseCase) Remove(serviceName string) error {
	w := uc.Out

	isIgnored, err := ignore.IsIgnored(serviceName)
	if err != nil {
		return errors.New(errors.ErrCodeWorkspaceError, i18n.T("error.ignore_check")).WithError(err)
	}

	if !isIgnored {
		fmt.Fprintf(w, "ℹ️  %s\n", i18n.T("output.ignore_not_in_list", serviceName))
		return nil
	}

	if err := ignore.RemoveService(serviceName); err != nil {
		return errors.New(errors.ErrCodeWorkspaceError, i18n.T("error.ignore_remove")).WithError(err)
	}

	fmt.Fprintf(w, "✔ %s\n", i18n.T("output.ignore_removed", serviceName))
	fmt.Fprintf(w, "ℹ️  %s\n", i18n.T("output.ignore_next_up_normal"))

	return nil
}

// List lists all ignored services
func (uc *IgnoreUseCase) List() error {
	w := uc.Out

	ignoredServices, err := ignore.GetIgnoredServices()
	if err != nil {
		return errors.New(errors.ErrCodeWorkspaceError, i18n.T("error.ignore_list")).WithError(err)
	}

	if len(ignoredServices) == 0 {
		fmt.Fprintln(w, i18n.T("output.ignore_empty_list"))
		return nil
	}

	fmt.Fprintln(w, i18n.T("output.ignore_list_header"))
	for _, name := range ignoredServices {
		fmt.Fprintf(w, "  - %s\n", name)
	}

	return nil
}
