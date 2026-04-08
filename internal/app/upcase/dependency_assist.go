package upcase

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"raioz/internal/audit"
	"raioz/internal/config"
	"raioz/internal/domain/interfaces"
	"raioz/internal/i18n"
	"raioz/internal/output"
)

// handleDependencyAssist handles dependency resolution assist mode
// Returns true if user wants to continue, false if should abort
// Also returns list of services added via dependency assist for metadata tracking
func (uc *UseCase) handleDependencyAssist(deps *config.Deps, ws *interfaces.Workspace, dryRun bool) (bool, []string, error) {
	// Create service path resolver function
	servicePathResolver := func(name string, svc config.Service) string {
		return uc.deps.Workspace.GetServicePath(ws, name, svc)
	}

	// Detect missing dependencies
	missing, err := uc.deps.ConfigLoader.DetectMissingDependencies(deps, servicePathResolver)
	if err != nil {
		return false, nil, fmt.Errorf("failed to detect missing dependencies: %w", err)
	}

	if len(missing) == 0 {
		// No missing dependencies, continue normally
		return true, []string{}, nil
	}

	// Group missing dependencies by service
	missingByService := make(map[string][]config.MissingDependency)
	for _, dep := range missing {
		missingByService[dep.RequiredBy] = append(missingByService[dep.RequiredBy], dep)
	}

	// Display missing dependencies
	output.PrintWarning(i18n.T("app.missing_deps_header"))
	output.PrintInfo("")
	for _, depsList := range missingByService {
		for _, dep := range depsList {
			output.PrintInfo(i18n.T("up.dep.service_detail", dep.ServiceName))
			output.PrintInfo(i18n.T("up.dep.required_by", dep.RequiredBy))
			if dep.FoundPath != "" {
				output.PrintInfo(i18n.T("up.dep.found_definition", dep.FoundPath))
			}
			if dep.FoundConfig != nil {
				output.PrintInfo(i18n.T("up.dep.definition_detail",
					dep.FoundConfig.Source.Kind,
					dep.FoundConfig.Source.Repo,
					dep.FoundConfig.Source.Branch,
				))
			} else {
				output.PrintInfo(i18n.T("up.dep.definition_not_found"))
			}
			output.PrintInfo("")
		}
	}

	if dryRun {
		// Dry-run mode: just show what would be done
		output.PrintInfo(i18n.T("app.missing_deps_dry_run"))
		return false, []string{}, nil // Abort in dry-run mode
	}

	// Interactive mode: ask user what to do
	output.PrintInfo(i18n.T("app.missing_deps_choose"))
	output.PrintInfo(i18n.T("app.missing_deps_opt_add"))
	output.PrintInfo(i18n.T("app.missing_deps_opt_ignore"))
	output.PrintInfo(i18n.T("app.missing_deps_opt_stub"))
	output.PrintInfo("")

	var servicesToAdd []config.MissingDependency
	var servicesToIgnore []string

	reader := bufio.NewReader(os.Stdin)
	for _, dep := range missing {
		output.PrintPrompt(i18n.T("up.dep.dependency_prompt", dep.ServiceName, dep.RequiredBy))
		if dep.FoundConfig != nil {
			output.PrintPrompt(i18n.T("up.dep.add_ignore_stub_default_add"))
		} else {
			output.PrintPrompt(i18n.T("up.dep.ignore_stub_default_ignore"))
		}

		input, err := reader.ReadString('\n')
		if err != nil {
			return false, nil, fmt.Errorf("failed to read input: %w", err)
		}

		input = strings.TrimSpace(input)
		if input == "" {
			// Default action
			if dep.FoundConfig != nil {
				input = "1"
			} else {
				input = "2"
			}
		}

		choice, err := strconv.Atoi(input)
		if err != nil || choice < 1 || choice > 3 {
			output.PrintWarning(i18n.T("up.dep.invalid_choice_ignoring", input))
			servicesToIgnore = append(servicesToIgnore, dep.ServiceName)
			continue
		}

		switch choice {
		case 1:
			// Add to root workspace
			if dep.FoundConfig == nil {
				output.PrintWarning(i18n.T("app.missing_deps_cannot_add", dep.ServiceName))
				servicesToIgnore = append(servicesToIgnore, dep.ServiceName)
			} else {
				servicesToAdd = append(servicesToAdd, dep)
			}
		case 2:
			// Ignore
			servicesToIgnore = append(servicesToIgnore, dep.ServiceName)
		case 3:
			// Add as stub (not implemented yet, treat as ignore)
			output.PrintInfo(i18n.T("app.missing_deps_stub_not_impl", dep.ServiceName))
			servicesToIgnore = append(servicesToIgnore, dep.ServiceName)
		}
	}

	// Track services added for metadata
	var addedServices []string

	// Add services to root config
	if len(servicesToAdd) > 0 {
		output.PrintProgress(i18n.T("app.missing_deps_adding"))
		for _, dep := range servicesToAdd {
			// Copy service config from found config
			if deps.Services == nil {
				deps.Services = make(map[string]config.Service)
			}

			// Add service with origin and addedBy metadata (stored in raioz.root.json)
			newSvc := *dep.FoundConfig
			deps.Services[dep.ServiceName] = newSvc
			addedServices = append(addedServices, dep.ServiceName)

			// Log audit event
			reason := fmt.Sprintf("dependency assist: required by %s", dep.RequiredBy)
			if err := audit.LogServiceAssisted(dep.ServiceName, dep.RequiredBy, reason); err != nil {
				// Log audit error but don't fail
				output.PrintWarning(i18n.T("output.failed_log_audit", err))
			}

			output.PrintSuccess(i18n.T("up.dep.added_origin", dep.ServiceName, dep.RequiredBy))
		}
	}

	// Show ignored services
	if len(servicesToIgnore) > 0 {
		output.PrintWarning(i18n.T("app.missing_deps_ignored", len(servicesToIgnore), servicesToIgnore))
		output.PrintInfo(i18n.T("app.missing_deps_ignored_warn"))
	}

	return true, addedServices, nil
}

// handleDependencyConflicts handles dependency conflicts
// Returns true if user wants to continue, false if should abort
// Also returns list of conflict resolutions for audit logging
func (uc *UseCase) handleDependencyConflicts(deps *config.Deps, ws *interfaces.Workspace, dryRun bool) (bool, []string, error) {
	// Create service path resolver function
	servicePathResolver := func(name string, svc config.Service) string {
		return uc.deps.Workspace.GetServicePath(ws, name, svc)
	}

	conflicts, err := uc.deps.ConfigLoader.DetectDependencyConflicts(deps, servicePathResolver)
	if err != nil {
		return false, nil, fmt.Errorf("failed to detect dependency conflicts: %w", err)
	}

	if len(conflicts) == 0 {
		// No conflicts, continue normally
		return true, []string{}, nil
	}

	// Display conflicts
	output.PrintWarning(i18n.T("app.conflict_header"))
	output.PrintInfo("")
	for _, conflict := range conflicts {
		output.PrintInfo(i18n.T("up.dep.conflict_service", conflict.ServiceName))
		output.PrintInfo(i18n.T("up.dep.conflict_differences"))
		for _, diff := range conflict.Differences {
			output.PrintInfo(i18n.T("up.dep.conflict_diff_item", diff))
		}
		output.PrintInfo("")
	}

	if dryRun {
		// Dry-run mode: just show what would be done
		output.PrintInfo(i18n.T("app.conflict_dry_run"))
		return false, []string{}, nil // Abort in dry-run mode
	}

	// Interactive mode: ask user what to do
	output.PrintInfo(i18n.T("app.conflict_choose"))
	output.PrintInfo(i18n.T("app.conflict_opt_keep"))
	output.PrintInfo(i18n.T("app.conflict_opt_replace"))
	output.PrintInfo(i18n.T("app.conflict_opt_abort"))
	output.PrintInfo("")

	reader := bufio.NewReader(os.Stdin)
	var shouldAbort bool
	var resolutions []string

	for _, conflict := range conflicts {
		output.PrintPrompt(i18n.T("up.dep.conflict_prompt", conflict.ServiceName))

		input, err := reader.ReadString('\n')
		if err != nil {
			return false, nil, fmt.Errorf("failed to read input: %w", err)
		}

		input = strings.TrimSpace(input)
		if input == "" {
			input = "1" // Default: keep root
		}

		choice, err := strconv.Atoi(input)
		if err != nil || choice < 1 || choice > 3 {
			output.PrintWarning(i18n.T("up.dep.conflict_invalid_keeping", input))
			continue
		}

		resolution := ""
		switch choice {
		case 1:
			// Keep root (do nothing)
			resolution = "keep"
			output.PrintSuccess(i18n.T("up.dep.conflict_keeping_root", conflict.ServiceName))
		case 2:
			// Replace root with service config
			if conflict.ServiceConfig != nil {
				deps.Services[conflict.ServiceName] = *conflict.ServiceConfig
				resolution = "replace"
				output.PrintSuccess(i18n.T("up.dep.conflict_replaced", conflict.ServiceName))
			}
		case 3:
			// Abort
			shouldAbort = true
			output.PrintWarning(i18n.T("up.dep.conflict_aborting", conflict.ServiceName))
		}

		if resolution != "" {
			// Log audit event
			reason := fmt.Sprintf("conflict resolution: %s (differences: %v)", resolution, conflict.Differences)
			if err := audit.LogConflictResolved(conflict.ServiceName, resolution, reason); err != nil {
				// Log audit error but don't fail
				output.PrintWarning(i18n.T("output.failed_log_audit", err))
			}
			resolutions = append(resolutions, fmt.Sprintf("%s:%s", conflict.ServiceName, resolution))
		}
	}

	if shouldAbort {
		return false, resolutions, fmt.Errorf("aborted due to dependency conflicts")
	}

	return true, resolutions, nil
}
