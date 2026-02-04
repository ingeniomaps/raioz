package upcase

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"raioz/internal/config"
	"raioz/internal/docker"
	"raioz/internal/logging"
	"raioz/internal/output"
	"raioz/internal/state"
	"raioz/internal/workspace"
)

// ServiceConflict represents a detected conflict for a service
type ServiceConflict struct {
	ServiceName      string                   // Name of the service (e.g., "nginx")
	ConflictType     string                   // "cloned_running" | "local_running" | "preference"
	CurrentProject   string                   // Project name where service is currently running
	CurrentLocation  string                   // Path or location description
	CurrentContainer string                   // Container name if running
	CurrentSource    string                   // "git" | "local" | "image"
	TargetLocation   string                   // Where we want to run it from
	TargetContainer  string                   // Container name that would be created
	Preference       *state.ServicePreference // Saved preference if exists
}

// detectServiceConflict detects if a service is already running and would conflict
func (uc *UseCase) detectServiceConflict(
	ctx context.Context,
	serviceName string,
	deps *config.Deps,
	ws *workspace.Workspace,
	projectDir string,
	isLocalProject bool,
) (*ServiceConflict, error) {
	workspaceName := deps.GetWorkspaceName()

	// Check if service has a saved preference
	pref, err := state.GetServicePreference(ws, serviceName)
	if err != nil {
		logging.WarnWithContext(ctx, "Failed to load service preference", "service", serviceName, "error", err.Error())
	}

	// Check if service is running from workspace (cloned service)
	// First, check if there's a workspace compose file and if service is running there
	workspaceComposePath := workspace.GetComposePath(ws)
	if workspaceComposePath != "" {
		containerName, err := docker.GetContainerNameWithContext(ctx, workspaceComposePath, serviceName)
		if err == nil && containerName != "" {
			// Service is running from workspace - find which project owns it
			currentProject := deps.Project.Name
			// Try to find from global state
			globalState, err := state.LoadGlobalState()
			if err == nil {
				// Look for project that has this service running
				for projName, projState := range globalState.Projects {
					for _, svcState := range projState.Services {
						if svcState.Name == serviceName && svcState.Status == "running" {
							currentProject = projName
							break
						}
					}
					if currentProject != deps.Project.Name {
						break
					}
				}
			}

			return &ServiceConflict{
				ServiceName:      serviceName,
				ConflictType:     "cloned_running",
				CurrentProject:   currentProject,
				CurrentLocation:  fmt.Sprintf("workspace (cloned service)"),
				CurrentContainer: containerName,
				CurrentSource:    "git",
				TargetLocation:   projectDir,
				TargetContainer:  serviceName, // Will be determined by compose or normalized
				Preference:       pref,
			}, nil
		}
	}

	// Check if service is running from a local project (via docker ps)
	// Only conflict when the running container is from OUR workspace (same workspace we're deploying to).
	// Containers from other workspaces (e.g. nunzio-nginx when deploying to roax) do not conflict — they coexist.
	if isLocalProject {
		cmd := exec.CommandContext(ctx, "docker", "ps", "--format", "{{.Names}}\t{{.Image}}")
		output, err := cmd.Output()
		if err == nil {
			lines := strings.Split(strings.TrimSpace(string(output)), "\n")
			workspacePrefix := fmt.Sprintf("%s-", workspaceName)
			for _, line := range lines {
				if line == "" {
					continue
				}
				parts := strings.Split(line, "\t")
				if len(parts) < 1 {
					continue
				}
				containerName := parts[0]

				// Only conflict if this container is from our workspace (same name we would use)
				if containerName == serviceName || strings.HasSuffix(containerName, "-"+serviceName) {
					if strings.HasPrefix(containerName, workspacePrefix) {
						return &ServiceConflict{
							ServiceName:      serviceName,
							ConflictType:     "local_running",
							CurrentProject:   "local",
							CurrentLocation:  projectDir,
							CurrentContainer: containerName,
							CurrentSource:    "local",
							TargetLocation:   fmt.Sprintf("workspace (would clone)"),
							TargetContainer: func() string {
								name, _ := docker.NormalizeContainerName(workspaceName, serviceName, workspaceName, true)
								return name
							}(),
							Preference: pref,
						}, nil
					}
					// Container from another workspace (e.g. nunzio-nginx) — no conflict, skip
				}
			}
		}
	}

	// Check if there's a preference that conflicts
	if pref != nil && pref.Preference != "ask" {
		// If preference is "local" but we're trying to run from workspace
		if pref.Preference == "local" && !isLocalProject {
			return &ServiceConflict{
				ServiceName:      serviceName,
				ConflictType:     "preference",
				CurrentProject:   "preference",
				CurrentLocation:  pref.ProjectPath,
				CurrentContainer: serviceName, // Estimated
				CurrentSource:    "local",
				TargetLocation:   fmt.Sprintf("workspace (would clone)"),
				TargetContainer: func() string {
					name, _ := docker.NormalizeContainerName(workspaceName, serviceName, workspaceName, true)
					return name
				}(),
				Preference: pref,
			}, nil
		}
		// If preference is "cloned" but we're trying to run from local project
		if pref.Preference == "cloned" && isLocalProject {
			containerName, _ := docker.NormalizeContainerName(pref.Workspace, serviceName, pref.Workspace, true)
			return &ServiceConflict{
				ServiceName:      serviceName,
				ConflictType:     "preference",
				CurrentProject:   pref.Workspace,
				CurrentLocation:  fmt.Sprintf("workspace"),
				CurrentContainer: containerName,
				CurrentSource:    "git",
				TargetLocation:   projectDir,
				TargetContainer:  serviceName,
				Preference:       pref,
			}, nil
		}
	}

	// No conflict detected
	return nil, nil
}

// resolveServiceConflict handles a service conflict by asking the user
func (uc *UseCase) resolveServiceConflict(
	ctx context.Context,
	conflict *ServiceConflict,
	isLocalProject bool,
	workspaceName string,
	projectDir string,
) (string, error) {
	// Check if there's a preference that doesn't require asking
	if conflict.Preference != nil && conflict.Preference.Preference != "ask" {
		// Apply preference automatically
		logging.InfoWithContext(ctx, "Applying saved preference for service",
			"service", conflict.ServiceName,
			"preference", conflict.Preference.Preference,
		)
		return conflict.Preference.Preference, nil
	}

	// Show conflict information
	output.PrintWarning(fmt.Sprintf("⚠️  Conflict detected: service '%s' is already running", conflict.ServiceName))
	output.PrintInfo("")
	output.PrintInfo(fmt.Sprintf("Current status:"))
	output.PrintInfo(fmt.Sprintf("  Container: %s", conflict.CurrentContainer))
	output.PrintInfo(fmt.Sprintf("  Source: %s (%s)", conflict.CurrentSource, conflict.CurrentLocation))
	if conflict.CurrentProject != "" && conflict.CurrentProject != "preference" {
		output.PrintInfo(fmt.Sprintf("  Project: %s", conflict.CurrentProject))
	}
	output.PrintInfo("")
	output.PrintInfo(fmt.Sprintf("Your project wants to run from:"))
	output.PrintInfo(fmt.Sprintf("  Location: %s", conflict.TargetLocation))
	output.PrintInfo(fmt.Sprintf("  Container: %s", conflict.TargetContainer))
	output.PrintInfo("")

	// Determine options based on conflict type
	var options []string
	if conflict.ConflictType == "cloned_running" || conflict.ConflictType == "preference" {
		// Service is running from workspace, we want to run local
		options = []string{
			"Stop cloned service and use local project (recommended for development)",
			"Keep cloned service, skip local project",
			"Cancel operation",
		}
	} else {
		// Service is running locally, we want to clone it
		options = []string{
			"Stop local project and use cloned service",
			"Keep local project, skip cloned service in this run",
			"Update preference to always use local project",
			"Cancel operation",
		}
	}

	// Show options
	for i, opt := range options {
		fmt.Printf("  [%d] %s\n", i+1, opt)
	}
	fmt.Print("\nYour choice [1-", len(options), "]: ")

	// Read user input
	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("failed to read user response: %w", err)
	}

	response = strings.TrimSpace(response)
	var choice int
	if _, err := fmt.Sscanf(response, "%d", &choice); err != nil || choice < 1 || choice > len(options) {
		return "", fmt.Errorf("invalid choice: %s", response)
	}

	// Process choice
	if conflict.ConflictType == "cloned_running" || conflict.ConflictType == "preference" {
		switch choice {
		case 1:
			// Stop cloned and use local
			return "local", nil
		case 2:
			// Keep cloned, skip local
			return "skip", nil
		case 3:
			// Cancel
			return "cancel", nil
		}
	} else {
		switch choice {
		case 1:
			// Stop local and use cloned
			return "cloned", nil
		case 2:
			// Keep local, skip cloned
			return "skip", nil
		case 3:
			// Update preference to always use local
			return "local_pref", nil
		case 4:
			// Cancel
			return "cancel", nil
		}
	}

	return "", fmt.Errorf("unexpected choice: %d", choice)
}

// applyServiceConflictResolution applies the resolution decision
func (uc *UseCase) applyServiceConflictResolution(
	ctx context.Context,
	conflict *ServiceConflict,
	resolution string,
	serviceName string,
	deps *config.Deps,
	ws *workspace.Workspace,
	projectDir string,
	isLocalProject bool,
) error {
	workspaceName := deps.GetWorkspaceName()

	switch resolution {
	case "local":
		// Stop only the conflicting service from workspace (do not bring down infra or other services)
		output.PrintInfo("Stopping cloned service from workspace...")
		composePath := workspace.GetComposePath(ws)
		if composePath != "" {
			if err := uc.deps.DockerRunner.StopServiceWithContext(ctx, composePath, serviceName); err != nil {
				logging.WarnWithContext(ctx, "Failed to stop service from workspace", "error", err.Error())
			}
		}
		// Save preference
		pref := state.ServicePreference{
			ServiceName: serviceName,
			Preference:  "local",
			ProjectPath: projectDir,
			Workspace:   workspaceName,
			Reason:      "User chose local project over cloned service",
			Timestamp:   time.Now(),
		}
		if err := state.SetServicePreference(ws, pref); err != nil {
			logging.WarnWithContext(ctx, "Failed to save service preference", "error", err.Error())
		}
		output.PrintSuccess("Preference saved: use local project for this service")
		return nil

	case "cloned":
		// Stop local project (need to find and stop its compose)
		output.PrintInfo("Stopping local project...")
		// Try to find compose file in projectDir
		possiblePaths := []string{
			fmt.Sprintf("%s/docker-compose.yml", projectDir),
			fmt.Sprintf("%s/docker/docker-compose.yml", projectDir),
			fmt.Sprintf("%s/compose.yml", projectDir),
		}
		for _, path := range possiblePaths {
			if _, err := os.Stat(path); err == nil {
				if err := uc.deps.DockerRunner.DownWithContext(ctx, path); err != nil {
					logging.WarnWithContext(ctx, "Failed to stop local project", "error", err.Error())
				}
				break
			}
		}
		// Save preference
		pref := state.ServicePreference{
			ServiceName: serviceName,
			Preference:  "cloned",
			Workspace:   workspaceName,
			Reason:      "User chose cloned service over local project",
			Timestamp:   time.Now(),
		}
		if err := state.SetServicePreference(ws, pref); err != nil {
			logging.WarnWithContext(ctx, "Failed to save service preference", "error", err.Error())
		}
		output.PrintSuccess("Preference saved: use cloned service for this service")
		return nil

	case "local_pref":
		// Save preference to always use local
		pref := state.ServicePreference{
			ServiceName: serviceName,
			Preference:  "local",
			ProjectPath: projectDir,
			Workspace:   workspaceName,
			Reason:      "User set preference to always use local project",
			Timestamp:   time.Now(),
		}
		if err := state.SetServicePreference(ws, pref); err != nil {
			return fmt.Errorf("failed to save service preference: %w", err)
		}
		output.PrintSuccess("Preference saved: always use local project for this service")
		return nil

	case "skip":
		output.PrintInfo("Keeping current service, skipping...")
		return fmt.Errorf("service conflict: user chose to skip")

	case "cancel":
		output.PrintInfo("Operation cancelled by user")
		return fmt.Errorf("operation cancelled by user")

	default:
		return fmt.Errorf("unknown resolution: %s", resolution)
	}
}
