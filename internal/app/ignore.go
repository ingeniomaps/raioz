package app

import (
	"fmt"

	"raioz/internal/i18n"
	"raioz/internal/ignore"
	"raioz/internal/output"
)

// IgnoreUseCase handles ignore operations for services
type IgnoreUseCase struct {
	deps *Dependencies
}

// NewIgnoreUseCase creates a new IgnoreUseCase
func NewIgnoreUseCase(deps *Dependencies) *IgnoreUseCase {
	return &IgnoreUseCase{deps: deps}
}

// Add adds a service to the ignore list
func (uc *IgnoreUseCase) Add(serviceName string, configPath string) error {
	if serviceName == "" {
		return fmt.Errorf("service name cannot be empty")
	}

	// Check if service is already ignored
	isIgnored, err := ignore.IsIgnored(serviceName)
	if err != nil {
		return fmt.Errorf("failed to check if service is ignored: %w", err)
	}

	if isIgnored {
		output.PrintInfo(i18n.T("output.ignore_already_ignored", serviceName))
		return nil
	}

	// Add to ignore list
	if err := ignore.AddService(serviceName); err != nil {
		return fmt.Errorf("failed to add service to ignore list: %w", err)
	}

	output.PrintSuccess(i18n.T("output.ignore_added", serviceName))
	output.PrintInfo(i18n.T("output.ignore_next_up"))

	// Check if service exists in current config and warn about dependencies
	deps, _, _ := uc.deps.ConfigLoader.LoadDeps(configPath)
	if deps != nil {
		if _, exists := deps.Services[serviceName]; exists {
			var dependents []string
			for name, svc := range deps.Services {
				for _, dep := range svc.Docker.DependsOn {
					if dep == serviceName {
						dependents = append(dependents, name)
						break
					}
				}
			}
			if len(dependents) > 0 {
				output.PrintWarning(
					i18n.T("output.ignore_dependents_warning", serviceName, dependents),
				)
			}
		}
	}

	return nil
}

// Remove removes a service from the ignore list
func (uc *IgnoreUseCase) Remove(serviceName string) error {
	// Check if service is ignored
	isIgnored, err := ignore.IsIgnored(serviceName)
	if err != nil {
		return fmt.Errorf("failed to check if service is ignored: %w", err)
	}

	if !isIgnored {
		output.PrintInfo(i18n.T("output.ignore_not_in_list", serviceName))
		return nil
	}

	// Remove from ignore list
	if err := ignore.RemoveService(serviceName); err != nil {
		return fmt.Errorf("failed to remove service from ignore list: %w", err)
	}

	output.PrintSuccess(i18n.T("output.ignore_removed", serviceName))
	output.PrintInfo(i18n.T("output.ignore_next_up_normal"))

	return nil
}

// List lists all ignored services
func (uc *IgnoreUseCase) List() error {
	ignoredServices, err := ignore.GetIgnoredServices()
	if err != nil {
		return fmt.Errorf("failed to get ignored services: %w", err)
	}

	if len(ignoredServices) == 0 {
		fmt.Println(i18n.T("output.ignore_empty_list"))
		return nil
	}

	fmt.Println(i18n.T("output.ignore_list_header"))
	for _, name := range ignoredServices {
		fmt.Printf("  - %s\n", name)
	}

	return nil
}
