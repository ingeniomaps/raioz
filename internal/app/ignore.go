package app

import (
	"fmt"
	"io"
	"os"

	"raioz/internal/domain/models"
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

	// The ignore belongs to the project the command runs in, and names a
	// service that project declares: a typo ignored nothing and said ok.
	project, cfgDeps := uc.project(configPath)
	if cfgDeps != nil {
		if _, declared := cfgDeps.Services[serviceName]; !declared {
			return errors.New(
				errors.ErrCodeInvalidField,
				i18n.T("error.ignore_unknown_service", serviceName),
			).WithSuggestion(i18n.T("error.ignore_unknown_service_suggestion", joinServiceNames(cfgDeps)))
		}
	}

	ignored, err := ignore.ForProject(project)
	if err != nil {
		return errors.New(errors.ErrCodeWorkspaceError, i18n.T("error.ignore_check")).WithError(err)
	}

	if containsString(ignored, serviceName) {
		fmt.Fprintf(w, "ℹ️  %s\n", i18n.T("output.ignore_already_ignored", serviceName))
		return nil
	}

	if err := ignore.AddFor(project, serviceName); err != nil {
		return errors.New(errors.ErrCodeWorkspaceError, i18n.T("error.ignore_add")).WithError(err)
	}

	fmt.Fprintf(w, "✔ %s\n", i18n.T("output.ignore_added", serviceName))
	fmt.Fprintf(w, "ℹ️  %s\n", i18n.T("output.ignore_next_up"))

	if deps := cfgDeps; deps != nil {
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
func findDependents(deps *models.Deps, serviceName string) []string {
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
func (uc *IgnoreUseCase) Remove(serviceName string, configPath string) error {
	w := uc.Out
	project, _ := uc.project(configPath)

	ignored, err := ignore.ForProject(project)
	if err != nil {
		return errors.New(errors.ErrCodeWorkspaceError, i18n.T("error.ignore_check")).WithError(err)
	}

	if !containsString(ignored, serviceName) {
		fmt.Fprintf(w, "ℹ️  %s\n", i18n.T("output.ignore_not_in_list", serviceName))
		return nil
	}

	if err := ignore.RemoveFor(project, serviceName); err != nil {
		return errors.New(errors.ErrCodeWorkspaceError, i18n.T("error.ignore_remove")).WithError(err)
	}

	fmt.Fprintf(w, "✔ %s\n", i18n.T("output.ignore_removed", serviceName))
	fmt.Fprintf(w, "ℹ️  %s\n", i18n.T("output.ignore_next_up_normal"))

	return nil
}

// List lists all ignored services
func (uc *IgnoreUseCase) List(configPath string) error {
	w := uc.Out
	project, _ := uc.project(configPath)

	ignoredServices, err := ignore.ForProject(project)
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

// project returns the name and config of the project the command runs in.
// With no loadable config the name is empty, which addresses the legacy
// global list.
func (uc *IgnoreUseCase) project(configPath string) (string, *models.Deps) {
	if uc.deps == nil || uc.deps.ConfigLoader == nil {
		return "", nil
	}
	deps, _, err := uc.deps.ConfigLoader.LoadDeps(configPath)
	if err != nil || deps == nil {
		return "", nil
	}
	return deps.Project.Name, deps
}

func containsString(list []string, name string) bool {
	for _, item := range list {
		if item == name {
			return true
		}
	}
	return false
}
