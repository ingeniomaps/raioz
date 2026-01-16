package upcase

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"raioz/internal/audit"
	"raioz/internal/config"
	"raioz/internal/output"
	"raioz/internal/workspace"
)

// handleDependencyAssist handles dependency resolution assist mode
// Returns true if user wants to continue, false if should abort
// Also returns list of services added via dependency assist for metadata tracking
func (uc *UseCase) handleDependencyAssist(deps *config.Deps, ws *workspace.Workspace, dryRun bool) (bool, []string, error) {
	// Create service path resolver function
	servicePathResolver := func(name string, svc config.Service) string {
		return workspace.GetServicePath(ws, name, svc)
	}

	// Detect missing dependencies
	missing, err := config.DetectMissingDependencies(deps, servicePathResolver)
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
	fmt.Println("\n⚠️  Missing dependencies detected:")
	fmt.Println()
	for _, depsList := range missingByService {
		for _, dep := range depsList {
			fmt.Printf("  Service: %s\n", dep.ServiceName)
			fmt.Printf("  Required by: %s\n", dep.RequiredBy)
			if dep.FoundPath != "" {
				fmt.Printf("  Found definition in: %s\n", dep.FoundPath)
			}
			if dep.FoundConfig != nil {
				fmt.Printf("  Definition: mode=%s, repo=%s, branch=%s\n",
					dep.FoundConfig.Source.Kind,
					dep.FoundConfig.Source.Repo,
					dep.FoundConfig.Source.Branch,
				)
			} else {
				fmt.Printf("  Definition: (not found)\n")
			}
			fmt.Println()
		}
	}

	if dryRun {
		// Dry-run mode: just show what would be done
		fmt.Println("ℹ️  Dry-run mode: dependencies shown but not added")
		return false, []string{}, nil // Abort in dry-run mode
	}

	// Interactive mode: ask user what to do
	fmt.Println("Choose action for each dependency:")
	fmt.Println("  [1] Add to root workspace")
	fmt.Println("  [2] Ignore (service will fail)")
	fmt.Println("  [3] Add as stub/missing")
	fmt.Println()

	var servicesToAdd []config.MissingDependency
	var servicesToIgnore []string

	reader := bufio.NewReader(os.Stdin)
	for _, dep := range missing {
		fmt.Printf("Dependency '%s' (required by '%s'): ", dep.ServiceName, dep.RequiredBy)
		if dep.FoundConfig != nil {
			fmt.Printf("[1] Add / [2] Ignore / [3] Stub (default: 1): ")
		} else {
			fmt.Printf("[2] Ignore / [3] Stub (default: 2): ")
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
			fmt.Printf("⚠️  Invalid choice '%s', ignoring dependency\n", input)
			servicesToIgnore = append(servicesToIgnore, dep.ServiceName)
			continue
		}

		switch choice {
		case 1:
			// Add to root workspace
			if dep.FoundConfig == nil {
				fmt.Printf("⚠️  Cannot add dependency '%s': no definition found\n", dep.ServiceName)
				servicesToIgnore = append(servicesToIgnore, dep.ServiceName)
			} else {
				servicesToAdd = append(servicesToAdd, dep)
			}
		case 2:
			// Ignore
			servicesToIgnore = append(servicesToIgnore, dep.ServiceName)
		case 3:
			// Add as stub (not implemented yet, treat as ignore)
			fmt.Printf("ℹ️  Stub mode not implemented yet, ignoring dependency '%s'\n", dep.ServiceName)
			servicesToIgnore = append(servicesToIgnore, dep.ServiceName)
		}
	}

	// Track services added for metadata
	var addedServices []string

	// Add services to root config
	if len(servicesToAdd) > 0 {
		fmt.Println("\n📝 Adding dependencies to root workspace...")
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

			fmt.Printf("  ✅ Added '%s' (origin: %s)\n", dep.ServiceName, dep.RequiredBy)
		}
	}

	// Show ignored services
	if len(servicesToIgnore) > 0 {
		fmt.Printf("\n⚠️  Ignored %d dependency(ies): %v\n", len(servicesToIgnore), servicesToIgnore)
		fmt.Println("   Services may fail if these dependencies are required")
	}

	return true, addedServices, nil
}

// handleDependencyConflicts handles dependency conflicts
// Returns true if user wants to continue, false if should abort
// Also returns list of conflict resolutions for audit logging
func (uc *UseCase) handleDependencyConflicts(deps *config.Deps, ws *workspace.Workspace, dryRun bool) (bool, []string, error) {
	// Create service path resolver function
	servicePathResolver := func(name string, svc config.Service) string {
		return workspace.GetServicePath(ws, name, svc)
	}

	conflicts, err := config.DetectDependencyConflicts(deps, servicePathResolver)
	if err != nil {
		return false, nil, fmt.Errorf("failed to detect dependency conflicts: %w", err)
	}

	if len(conflicts) == 0 {
		// No conflicts, continue normally
		return true, []string{}, nil
	}

	// Display conflicts
	fmt.Println("\n⚠️  Dependency conflicts detected:")
	fmt.Println()
	for _, conflict := range conflicts {
		fmt.Printf("  Service: %s\n", conflict.ServiceName)
		fmt.Printf("  Differences:\n")
		for _, diff := range conflict.Differences {
			fmt.Printf("    - %s\n", diff)
		}
		fmt.Println()
	}

	if dryRun {
		// Dry-run mode: just show what would be done
		fmt.Println("ℹ️  Dry-run mode: conflicts shown but not resolved")
		return false, []string{}, nil // Abort in dry-run mode
	}

	// Interactive mode: ask user what to do
	fmt.Println("Choose action for each conflict:")
	fmt.Println("  [1] Keep root (recommended)")
	fmt.Println("  [2] Replace root")
	fmt.Println("  [3] Abort")
	fmt.Println()

	reader := bufio.NewReader(os.Stdin)
	var shouldAbort bool
	var resolutions []string

	for _, conflict := range conflicts {
		fmt.Printf("Conflict for '%s': [1] Keep root / [2] Replace / [3] Abort (default: 1): ", conflict.ServiceName)

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
			fmt.Printf("⚠️  Invalid choice '%s', keeping root configuration\n", input)
			continue
		}

		resolution := ""
		switch choice {
		case 1:
			// Keep root (do nothing)
			resolution = "keep"
			fmt.Printf("  ✅ Keeping root configuration for '%s'\n", conflict.ServiceName)
		case 2:
			// Replace root with service config
			if conflict.ServiceConfig != nil {
				deps.Services[conflict.ServiceName] = *conflict.ServiceConfig
				resolution = "replace"
				fmt.Printf("  ✅ Replaced root configuration for '%s' with service config\n", conflict.ServiceName)
			}
		case 3:
			// Abort
			shouldAbort = true
			fmt.Printf("  ⚠️  Aborting due to conflict in '%s'\n", conflict.ServiceName)
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
