package upcase

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"raioz/internal/config"
	"raioz/internal/domain/interfaces"
	"raioz/internal/i18n"
	"raioz/internal/logging"
	"raioz/internal/output"
	"raioz/internal/state"
)

// checkDependencyProjects checks if any dependency matches a running project and handles replacement
func (uc *UseCase) checkDependencyProjects(ctx context.Context, deps *config.Deps) error {
	// Load global state to check for running projects
	globalState, err := uc.deps.StateManager.LoadGlobalState()
	if err != nil {
		// If we can't load global state, continue anyway
		logging.WarnWithContext(ctx, "Failed to load global state for dependency check", "error", err.Error())
		return nil
	}

	// Collect all dependency names from services (service-level and docker-level)
	dependencyNames := make(map[string]bool)
	for _, svc := range deps.Services {
		for _, dep := range svc.GetDependsOn() {
			dependencyNames[dep] = true
		}
	}

	// Check each dependency against running projects
	for depName := range dependencyNames {
		// Check if this dependency is a running project
		projectState, exists := globalState.Projects[depName]
		if !exists {
			continue
		}

		// Check if project is actually running (has recent execution and empty services means it's command-based)
		if len(projectState.Services) == 0 {
			// This is a command-based project that's running
			logging.InfoWithContext(ctx, "Dependency matches running project",
				"dependency", depName,
				"project", projectState.Name,
				"workspace", projectState.Workspace,
			)

			// Ask user if they want to replace it
			shouldReplace, err := askReplaceRunningProject(ctx, depName, projectState, uc.deps.StateManager)
			if err != nil {
				return err
			}

			if shouldReplace {
				// Stop the running project
				if err := uc.stopRunningProject(ctx, depName, projectState); err != nil {
					logging.WarnWithContext(ctx, "Failed to stop running project", "project", depName, "error", err.Error())
					// Continue anyway
				}
			} else {
				// User chose not to replace, record decision
				if err := recordUserDecision(depName, false, uc.deps.StateManager); err != nil {
					logging.WarnWithContext(ctx, "Failed to record user decision", "error", err.Error())
				}
				output.PrintInfo(i18n.T("up.dep.keeping_existing", depName))
			}
		}
	}

	return nil
}

// askReplaceRunningProject asks the user if they want to replace a running project
func askReplaceRunningProject(
	ctx context.Context, projectName string,
	projectState state.ProjectState, sm interfaces.StateManager,
) (bool, error) {
	// Check if user has a saved decision
	savedDecision, err := loadUserDecision(projectName, sm)
	if err == nil && savedDecision != nil {
		// User has a saved decision, use it
		return *savedDecision, nil
	}

	// No saved decision, ask user
	output.PrintWarning(i18n.T("up.dep.already_running_warning", projectName))
	output.PrintInfo(i18n.T("up.dep.project_workspace", projectState.Workspace))
	output.PrintInfo(i18n.T("up.dep.will_stop_existing"))
	output.PrintPrompt(i18n.T("up.dep.replace_prompt"))

	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false, fmt.Errorf("failed to read user response: %w", err)
	}

	response = strings.TrimSpace(strings.ToLower(response))
	switch response {
	case "yes", "y":
		return true, nil
	case "no", "n":
		// Record decision
		_ = recordUserDecision(projectName, false, sm)
		return false, nil
	case "always", "a":
		// Record decision to always replace
		_ = recordUserDecision(projectName, true, sm)
		return true, nil
	case "never":
		// Record decision to never replace
		_ = recordUserDecision(projectName, false, sm)
		return false, nil
	default:
		// Invalid response, default to no
		output.PrintInfo(i18n.T("up.dep.invalid_response"))
		_ = recordUserDecision(projectName, false, sm)
		return false, nil
	}
}

// stopRunningProject stops a running project by executing its down command
func (uc *UseCase) stopRunningProject(
	ctx context.Context, projectName string, projectState state.ProjectState,
) error {
	// Try to find the .raioz.json in the project workspace
	configPath := filepath.Join(projectState.Workspace, ".raioz.json")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Config file doesn't exist, can't stop it properly
		logging.WarnWithContext(ctx,
			"Cannot find .raioz.json for running project",
			"project", projectName, "workspace", projectState.Workspace)
		// Remove from global state anyway
		_ = uc.deps.StateManager.RemoveProject(projectName)
		return fmt.Errorf("config file not found for project %s", projectName)
	}

	// Load the project's config
	deps, _, err := uc.deps.ConfigLoader.LoadDeps(configPath)
	if err != nil {
		logging.WarnWithContext(ctx, "Failed to load config for running project",
			"project", projectName, "error", err.Error())
		// Remove from global state anyway
		_ = uc.deps.StateManager.RemoveProject(projectName)
		return fmt.Errorf("failed to load config for project %s: %w", projectName, err)
	}

	// Execute down command using the UseCase
	output.PrintInfo(i18n.T("up.dep.stopping_project", projectName))
	if err := uc.processLocalProject(ctx, configPath, deps, "down", nil); err != nil {
		logging.WarnWithContext(ctx, "Failed to stop running project", "project", projectName, "error", err.Error())
		// Continue anyway - might already be stopped
	}

	// Remove from global state
	if err := uc.deps.StateManager.RemoveProject(projectName); err != nil {
		logging.WarnWithContext(ctx, "Failed to remove project from global state",
			"project", projectName, "error", err.Error())
	}

	output.PrintSuccess(i18n.T("up.dep.project_stopped", projectName))
	return nil
}

// recordUserDecision records the user's decision about replacing a project
func recordUserDecision(projectName string, replace bool, sm interfaces.StateManager) error {
	// Get base directory
	baseDir, err := sm.GetGlobalStatePath()
	if err != nil {
		return err
	}
	baseDir = filepath.Dir(baseDir)

	// Create decisions file path
	decisionsPath := filepath.Join(baseDir, "decisions.json")

	// Load existing decisions
	decisions := make(map[string]bool)
	if data, err := os.ReadFile(decisionsPath); err == nil {
		// Try to parse existing decisions (simple JSON)
		// For now, we'll use a simple format: one line per decision
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				project := strings.TrimSpace(parts[0])
				decision := strings.TrimSpace(parts[1])
				decisions[project] = decision == "true" || decision == "replace"
			}
		}
	}

	// Update decision
	decisions[projectName] = replace

	// Save decisions (simple format for now)
	var lines []string
	lines = append(lines, "# User decisions for project replacement")
	lines = append(lines, "# Format: project-name: true|false")
	lines = append(lines, "")
	for project, decision := range decisions {
		lines = append(lines, fmt.Sprintf("%s: %v", project, decision))
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(decisionsPath), 0700); err != nil {
		return fmt.Errorf("failed to create decisions directory: %w", err)
	}

	// Write decisions file
	data := strings.Join(lines, "\n")
	if err := os.WriteFile(decisionsPath, []byte(data), 0600); err != nil {
		return fmt.Errorf("failed to write decisions file: %w", err)
	}

	return nil
}

// loadUserDecision loads a saved user decision for a project
func loadUserDecision(projectName string, sm interfaces.StateManager) (*bool, error) {
	// Get base directory
	baseDir, err := sm.GetGlobalStatePath()
	if err != nil {
		return nil, err
	}
	baseDir = filepath.Dir(baseDir)

	// Create decisions file path
	decisionsPath := filepath.Join(baseDir, "decisions.json")

	// Read decisions file
	data, err := os.ReadFile(decisionsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No decisions file, no saved decision
		}
		return nil, fmt.Errorf("read decisions file %q: %w", decisionsPath, err)
	}

	// Parse decisions (simple format)
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			project := strings.TrimSpace(parts[0])
			if project == projectName {
				decision := strings.TrimSpace(parts[1])
				replace := decision == "true" || decision == "replace"
				return &replace, nil
			}
		}
	}

	return nil, nil // No saved decision for this project
}
