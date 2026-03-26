package upcase

import (
	"fmt"
	"os"
	"strings"

	"raioz/internal/config"
	"raioz/internal/errors"
	"raioz/internal/ignore"
	"raioz/internal/output"
)

// applyFilters handles profile filtering, feature flags, mocks, and ignore list
func (uc *UseCase) applyFilters(deps *config.Deps, profile string) (*config.Deps, error) {
	// Load environment variables for feature flags
	envVars := make(map[string]string)
	for _, key := range os.Environ() {
		pair := strings.SplitN(key, "=", 2)
		if len(pair) == 2 {
			envVars[pair[0]] = pair[1]
		}
	}

	// Validate feature flags and mocks
	if err := uc.deps.ConfigLoader.ValidateFeatureFlags(deps); err != nil {
		return nil, errors.New(errors.ErrCodeInvalidConfig, "Feature flag or mock configuration validation failed").WithSuggestion("Check your configuration for feature flag and mock configuration errors.").WithError(err)
	}

	// Filter by profile first
	if profile != "" {
		deps = uc.deps.ConfigLoader.FilterByProfile(deps, profile)
		if len(deps.Services) == 0 && len(deps.Infra) == 0 {
			return nil, errors.New(errors.ErrCodeInvalidConfig, fmt.Sprintf("No services or infra found for profile '%s'", profile)).WithSuggestion("Check that services and/or infra have the profile assigned in your configuration.")
		}
		output.PrintInfo(fmt.Sprintf("Using profile: %s", profile))
	} else if len(deps.Profiles) > 0 {
		// Default profiles from config: raioz up without --profile uses these
		deps = uc.deps.ConfigLoader.FilterByProfiles(deps, deps.Profiles)
		if len(deps.Services) == 0 && len(deps.Infra) == 0 {
			return nil, errors.New(errors.ErrCodeInvalidConfig, fmt.Sprintf("No services or infra found for default profiles %v", deps.Profiles)).WithSuggestion("Check that services and/or infra have these profiles in your configuration.")
		}
		output.PrintInfo(fmt.Sprintf("Using default profiles: %s", strings.Join(deps.Profiles, ", ")))
	}

	// Apply feature flags and mocks
	originalServiceCount := len(deps.Services)
	var mockServices []string
	deps, mockServices = uc.deps.ConfigLoader.FilterByFeatureFlags(deps, profile, envVars)
	filteredCount := originalServiceCount - len(deps.Services)
	if filteredCount > 0 {
		output.PrintInfo(fmt.Sprintf("%d service(s) disabled by feature flags", filteredCount))
	}
	if len(mockServices) > 0 {
		for _, mockName := range mockServices {
			output.PrintInfo(fmt.Sprintf("Using mock for service: %s", mockName))
		}
	}

	// Filter ignored services (must check dependencies before filtering)
	ignoredServiceNames, err := ignore.GetIgnoredServices()
	if err != nil {
		return nil, errors.New(errors.ErrCodeWorkspaceError, "Failed to load ignored services list").WithSuggestion("Check that ~/.raioz/ignore.json is valid JSON. " + "You can try removing it and running 'raioz up' again.").WithError(err)
	}
	if len(ignoredServiceNames) > 0 {
		// Check if any services depend on ignored services (before filtering)
		ignoredDependencies := uc.deps.ConfigLoader.CheckIgnoredDependencies(deps, ignoredServiceNames)
		if len(ignoredDependencies) > 0 {
			output.PrintWarning("Some services depend on ignored services and may fail:")
			for serviceName, ignoredDeps := range ignoredDependencies {
				output.PrintWarning(fmt.Sprintf(" Service '%s' depends on ignored services: %v", serviceName, ignoredDeps))
			}
		}
		// Filter ignored services
		deps, ignoredServiceNames, err = uc.deps.ConfigLoader.FilterIgnoredServices(deps)
		if err != nil {
			return nil, errors.New(errors.ErrCodeInvalidConfig, "Failed to filter ignored services").WithSuggestion("Check your configuration for errors. " + "Verify that service names in ignore list match your configuration.").WithError(err)
		}
		if len(ignoredServiceNames) > 0 {
			output.PrintInfo(fmt.Sprintf("Ignoring %d service(s): %v", len(ignoredServiceNames), ignoredServiceNames))
		}
	}

	// Check if we have project commands as fallback
	hasProjectCommands := deps.Project.Commands != nil && (
		deps.Project.Commands.Up != "" ||
		(deps.Project.Commands.Dev != nil && deps.Project.Commands.Dev.Up != "") ||
		(deps.Project.Commands.Prod != nil && deps.Project.Commands.Prod.Up != ""))

	// Check if we have services, infra, or project commands
	hasServices := len(deps.Services) > 0
	hasInfra := len(deps.Infra) > 0

	// Only fail if everything is empty (no services, no infra, no project commands)
	if !hasServices && !hasInfra && !hasProjectCommands {
		return nil, errors.New(errors.ErrCodeInvalidConfig, "No services, infrastructure, or project commands configured").WithSuggestion("Configure at least one of the following: " +
			"services, infrastructure, or 'project.commands.up'. " +
			"If you have services but they're all filtered out, check feature flag configurations, environment variables, and ignore list.")
	}

	return deps, nil
}
