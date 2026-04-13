package upcase

import (
	"os"
	"strings"

	"raioz/internal/config"
	"raioz/internal/errors"
	"raioz/internal/i18n"
	"raioz/internal/ignore"
	"raioz/internal/output"
)

// applyFilters handles profile filtering, feature flags, mocks, ignore list, and --only selection
func (uc *UseCase) applyFilters(deps *config.Deps, profile string, only []string) (*config.Deps, error) {
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
		return nil, errors.New(
			errors.ErrCodeInvalidConfig,
			i18n.T("error.feature_flag_validation_failed"),
		).WithSuggestion(
			i18n.T("error.feature_flag_validation_suggestion"),
		).WithError(err)
	}

	// Filter by profile first
	if profile != "" {
		deps = uc.deps.ConfigLoader.FilterByProfile(deps, profile)
		if len(deps.Services) == 0 && len(deps.Infra) == 0 {
			return nil, errors.New(
				errors.ErrCodeInvalidConfig,
				i18n.T("error.no_services_for_profile", profile),
			).WithSuggestion(
				i18n.T("error.no_services_for_profile_suggestion"),
			)
		}
		output.PrintInfo(i18n.T("up.validate.using_profile", profile))
	} else if len(deps.Profiles) > 0 {
		// Default profiles from config: raioz up without --profile uses these
		deps = uc.deps.ConfigLoader.FilterByProfiles(deps, deps.Profiles)
		if len(deps.Services) == 0 && len(deps.Infra) == 0 {
			return nil, errors.New(
				errors.ErrCodeInvalidConfig,
				i18n.T("error.no_services_for_default_profiles", deps.Profiles),
			).WithSuggestion(
				i18n.T("error.no_services_for_default_profiles_suggestion"),
			)
		}
		output.PrintInfo(i18n.T("up.validate.using_default_profiles", strings.Join(deps.Profiles, ", ")))
	}

	// Apply feature flags and mocks
	originalServiceCount := len(deps.Services)
	var mockServices []string
	deps, mockServices = uc.deps.ConfigLoader.FilterByFeatureFlags(deps, profile, envVars)
	filteredCount := originalServiceCount - len(deps.Services)
	if filteredCount > 0 {
		output.PrintInfo(i18n.T("up.validate.disabled_by_flags", filteredCount))
	}
	if len(mockServices) > 0 {
		for _, mockName := range mockServices {
			output.PrintInfo(i18n.T("up.validate.using_mock", mockName))
		}
	}

	// Filter ignored services (must check dependencies before filtering)
	ignoredServiceNames, err := ignore.GetIgnoredServices()
	if err != nil {
		return nil, errors.New(
			errors.ErrCodeWorkspaceError,
			i18n.T("error.ignored_services_load_failed"),
		).WithSuggestion(
			i18n.T("error.ignored_services_load_suggestion"),
		).WithError(err)
	}
	if len(ignoredServiceNames) > 0 {
		// Check if any services depend on ignored services (before filtering)
		ignoredDependencies := uc.deps.ConfigLoader.CheckIgnoredDependencies(deps, ignoredServiceNames)
		if len(ignoredDependencies) > 0 {
			output.PrintWarning(i18n.T("up.validate.ignored_deps_warning"))
			for serviceName, ignoredDeps := range ignoredDependencies {
				output.PrintWarning(i18n.T("up.validate.ignored_dep_detail", serviceName, ignoredDeps))
			}
		}
		// Filter ignored services
		deps, ignoredServiceNames, err = uc.deps.ConfigLoader.FilterIgnoredServices(deps)
		if err != nil {
			return nil, errors.New(
				errors.ErrCodeInvalidConfig,
				i18n.T("error.ignored_services_filter_failed"),
			).WithSuggestion(
				i18n.T("error.ignored_services_filter_suggestion"),
			).WithError(err)
		}
		if len(ignoredServiceNames) > 0 {
			output.PrintInfo(i18n.T("up.validate.ignoring_services", len(ignoredServiceNames), ignoredServiceNames))
		}
	}

	// Filter by --only: select specific services + their transitive dependencies
	if len(only) > 0 {
		// Validate that requested services/infra exist
		for _, name := range only {
			_, isSvc := deps.Services[name]
			_, isInfra := deps.Infra[name]
			if !isSvc && !isInfra {
				return nil, errors.New(
					errors.ErrCodeInvalidField,
					i18n.T("error.only_service_not_found", name),
				).WithContext("service", name)
			}
		}
		// Resolve transitive dependencies
		svcNames, infraNames := config.ResolveDependencies(deps, only)
		deps = config.FilterByServices(deps, svcNames, infraNames)
		output.PrintInfo(i18n.T("up.validate.using_only", len(deps.Services)+len(deps.Infra), only))
	}

	// Check if we have project commands as fallback
	hasProjectCommands := deps.Project.Commands != nil && (deps.Project.Commands.Up != "" ||
		(deps.Project.Commands.Dev != nil && deps.Project.Commands.Dev.Up != "") ||
		(deps.Project.Commands.Prod != nil && deps.Project.Commands.Prod.Up != ""))

	// Check if we have services, infra, or project commands
	hasServices := len(deps.Services) > 0
	hasInfra := len(deps.Infra) > 0

	// Only fail if everything is empty (no services, no infra, no project commands)
	if !hasServices && !hasInfra && !hasProjectCommands {
		return nil, errors.New(
			errors.ErrCodeInvalidConfig,
			i18n.T("error.no_services_or_commands"),
		).WithSuggestion(
			i18n.T("error.no_services_or_commands_suggestion"),
		)
	}

	return deps, nil
}
