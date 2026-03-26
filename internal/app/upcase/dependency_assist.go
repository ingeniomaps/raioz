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
	output.PrintWarning("Missing dependencies detected:")
	output.PrintInfo("")
	for _, depsList := range missingByService {
		for _, dep := range depsList {
			output.PrintInfo(fmt.Sprintf("  Service: %s", dep.ServiceName))
			output.PrintInfo(fmt.Sprintf("  Required by: %s", dep.RequiredBy))
			if dep.FoundPath != "" {
				output.PrintInfo(fmt.Sprintf("  Found definition in: %s", dep.FoundPath))
			}
			if dep.FoundConfig != nil {
				output.PrintInfo(fmt.Sprintf("  Definition: mode=%s, repo=%s, branch=%s",
					dep.FoundConfig.Source.Kind,
					dep.FoundConfig.Source.Repo,
					dep.FoundConfig.Source.Branch,
				))
			} else {
				output.PrintInfo("  Definition: (not found)")
			}
			output.PrintInfo("")
		}
	}

	if dryRun {
		// Dry-run mode: just show what would be done
		output.PrintInfo("Dry-run mode: dependencies shown but not added")
		return false, []string{}, nil // Abort in dry-run mode
	}

	// Interactive mode: ask user what to do
	output.PrintInfo("Choose action for each dependency:")
	output.PrintInfo("  [1] Add to root workspace")
	output.PrintInfo("  [2] Ignore (service will fail)")
	output.PrintInfo("  [3] Add as stub/missing")
	output.PrintInfo("")

	var servicesToAdd []config.MissingDependency
	var servicesToIgnore []string

	reader := bufio.NewReader(os.Stdin)
	for _, dep := range missing {
		output.PrintPrompt(fmt.Sprintf("Dependency '%s' (required by '%s'): ", dep.ServiceName, dep.RequiredBy))
		if dep.FoundConfig != nil {
			output.PrintPrompt("[1] Add / [2] Ignore / [3] Stub (default: 1): ")
		} else {
			output.PrintPrompt("[2] Ignore / [3] Stub (default: 2): ")
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
			output.PrintWarning(fmt.Sprintf("Invalid choice '%s', ignoring dependency", input))
			servicesToIgnore = append(servicesToIgnore, dep.ServiceName)
			continue
		}

		switch choice {
		case 1:
			// Add to root workspace
			if dep.FoundConfig == nil {
				output.PrintWarning(fmt.Sprintf("Cannot add dependency '%s': no definition found", dep.ServiceName))
				servicesToIgnore = append(servicesToIgnore, dep.ServiceName)
			} else {
				servicesToAdd = append(servicesToAdd, dep)
			}
		case 2:
			// Ignore
			servicesToIgnore = append(servicesToIgnore, dep.ServiceName)
		case 3:
			// Add as stub (not implemented yet, treat as ignore)
			output.PrintInfo(fmt.Sprintf("Stub mode not implemented yet, ignoring dependency '%s'", dep.ServiceName))
			servicesToIgnore = append(servicesToIgnore, dep.ServiceName)
		}
	}

	// Track services added for metadata
	var addedServices []string

	// Add services to root config
	if len(servicesToAdd) > 0 {
		output.PrintProgress("Adding dependencies to root workspace")
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
				output.PrintWarning(fmt.Sprintf("Failed to log audit event: %v", err))
			}

			output.PrintSuccess(fmt.Sprintf("Added '%s' (origin: %s)", dep.ServiceName, dep.RequiredBy))
		}
	}

	// Show ignored services
	if len(servicesToIgnore) > 0 {
		output.PrintWarning(fmt.Sprintf("Ignored %d dependency(ies): %v", len(servicesToIgnore), servicesToIgnore))
		output.PrintInfo("Services may fail if these dependencies are required")
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
	output.PrintWarning("Dependency conflicts detected:")
	output.PrintInfo("")
	for _, conflict := range conflicts {
		output.PrintInfo(fmt.Sprintf("  Service: %s", conflict.ServiceName))
		output.PrintInfo("  Differences:")
		for _, diff := range conflict.Differences {
			output.PrintInfo(fmt.Sprintf("    - %s", diff))
		}
		output.PrintInfo("")
	}

	if dryRun {
		// Dry-run mode: just show what would be done
		output.PrintInfo("Dry-run mode: conflicts shown but not resolved")
		return false, []string{}, nil // Abort in dry-run mode
	}

	// Interactive mode: ask user what to do
	output.PrintInfo("Choose action for each conflict:")
	output.PrintInfo("  [1] Keep root (recommended)")
	output.PrintInfo("  [2] Replace root")
	output.PrintInfo("  [3] Abort")
	output.PrintInfo("")

	reader := bufio.NewReader(os.Stdin)
	var shouldAbort bool
	var resolutions []string

	for _, conflict := range conflicts {
		output.PrintPrompt(fmt.Sprintf("Conflict for '%s': [1] Keep root / [2] Replace / [3] Abort (default: 1): ", conflict.ServiceName))

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
			output.PrintWarning(fmt.Sprintf("Invalid choice '%s', keeping root configuration", input))
			continue
		}

		resolution := ""
		switch choice {
		case 1:
			// Keep root (do nothing)
			resolution = "keep"
			output.PrintSuccess(fmt.Sprintf("Keeping root configuration for '%s'", conflict.ServiceName))
		case 2:
			// Replace root with service config
			if conflict.ServiceConfig != nil {
				deps.Services[conflict.ServiceName] = *conflict.ServiceConfig
				resolution = "replace"
				output.PrintSuccess(fmt.Sprintf("Replaced root configuration for '%s' with service config", conflict.ServiceName))
			}
		case 3:
			// Abort
			shouldAbort = true
			output.PrintWarning(fmt.Sprintf("Aborting due to conflict in '%s'", conflict.ServiceName))
		}

		if resolution != "" {
			// Log audit event
			reason := fmt.Sprintf("conflict resolution: %s (differences: %v)", resolution, conflict.Differences)
			if err := audit.LogConflictResolved(conflict.ServiceName, resolution, reason); err != nil {
				// Log audit error but don't fail
				output.PrintWarning(fmt.Sprintf("Failed to log audit event: %v", err))
			}
			resolutions = append(resolutions, fmt.Sprintf("%s:%s", conflict.ServiceName, resolution))
		}
	}

	if shouldAbort {
		return false, resolutions, fmt.Errorf("aborted due to dependency conflicts")
	}

	return true, resolutions, nil
}
