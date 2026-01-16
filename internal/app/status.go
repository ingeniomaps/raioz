package app

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"raioz/internal/config"
	"raioz/internal/domain/interfaces"
	"raioz/internal/docker"
	"raioz/internal/errors"
	"raioz/internal/host"
	"raioz/internal/output"
	workspacepkg "raioz/internal/workspace"
)

// StatusOptions contains options for the Status use case
type StatusOptions struct {
	ProjectName string
	ConfigPath  string
	JSON        bool
}

// StatusUseCase handles the "status" use case - showing project status
type StatusUseCase struct {
	deps *Dependencies
}

// NewStatusUseCase creates a new StatusUseCase with injected dependencies
func NewStatusUseCase(deps *Dependencies) *StatusUseCase {
	return &StatusUseCase{
		deps: deps,
	}
}

// Execute executes the status use case
func (uc *StatusUseCase) Execute(ctx context.Context, opts StatusOptions) error {
	// Create context for the operation
	if ctx == nil {
		ctx = context.Background()
	}

	var ws *interfaces.Workspace
	var err error

	// Try to determine project name
	projectName := opts.ProjectName
	if projectName == "" {
		deps, _, _ := uc.deps.ConfigLoader.LoadDeps(opts.ConfigPath)
		if deps != nil {
			projectName = deps.Project.Name
		} else {
			return errors.New(
				errors.ErrCodeInvalidConfig,
				"Could not determine project name",
			).WithSuggestion(
				"Please provide --config or --project flag to specify the project.",
			)
		}
	}

	// Resolve workspace
	ws, err = uc.deps.Workspace.Resolve(projectName)
	if err != nil {
		return errors.New(
			errors.ErrCodeWorkspaceError,
			"Failed to resolve workspace",
		).WithSuggestion(
			"Check that the project name is correct. " +
				"Verify workspace directories exist and are accessible.",
		).WithContext("project", projectName).WithError(err)
	}

	// Check if state exists
	if !uc.deps.StateManager.Exists(ws) {
		if opts.JSON {
			fmt.Println("{}")
		} else {
			fmt.Println("Project is not running (no state file found)")
		}
		return nil
	}

	// Load original .raioz.json to check for disabled services
	originalDeps, _, err := uc.deps.ConfigLoader.LoadDeps(opts.ConfigPath)
	if err != nil {
		return errors.New(
			errors.ErrCodeInvalidConfig,
			"Failed to load .raioz.json",
		).WithSuggestion(
			"Ensure .raioz.json exists and is valid JSON. " +
				"Use --config flag to specify a different path if needed.",
		).WithError(err)
	}

	// Load state
	stateDeps, err := uc.deps.StateManager.Load(ws)
	if err != nil {
		return errors.New(
			errors.ErrCodeStateLoadError,
			"Failed to load project state",
		).WithSuggestion(
			"This may indicate a corrupted state file. " +
				"Try running 'raioz down' and then 'raioz up' again to recreate the state.",
		).WithContext("workspace", uc.deps.Workspace.GetRoot(ws)).WithError(err)
	}

	composePath := uc.deps.Workspace.GetComposePath(ws)

	// Collect all service names from state (running services)
	var serviceNames []string
	for name := range stateDeps.Services {
		serviceNames = append(serviceNames, name)
	}
	for name := range stateDeps.Infra {
		serviceNames = append(serviceNames, name)
	}

	// Find disabled services (in original deps but not in state)
	var disabledServices []string
	envVars := make(map[string]string)
	for _, key := range os.Environ() {
		pair := strings.SplitN(key, "=", 2)
		if len(pair) == 2 {
			envVars[pair[0]] = pair[1]
		}
	}
	for name, svc := range originalDeps.Services {
		// Check if service is disabled
		if !uc.deps.ConfigLoader.IsServiceEnabled(svc, "", envVars) {
			disabledServices = append(disabledServices, name)
		}
	}

	// Convert workspace to concrete type for host operations
	wsConcrete := (*workspacepkg.Workspace)(ws)

	// Load host processes state to check host services
	hostProcesses, err := host.LoadProcessesState(wsConcrete)
	if err != nil {
		// Log but continue - host processes state is optional
		hostProcesses = make(map[string]*host.ProcessInfo)
	}

	// Get detailed service info (for Docker services)
	servicesInfo, err := uc.deps.DockerRunner.GetServicesInfoWithContext(
		ctx,
		composePath,
		serviceNames,
		stateDeps.Project.Name,
		stateDeps.Services,
		ws,
	)
	if err != nil {
		return fmt.Errorf("failed to get services info: %w", err)
	}

	// Check host services status
	for _, name := range serviceNames {
		// Check if this is a host service (not in docker-compose.generated.yml)
		svc, exists := originalDeps.Services[name]
		if !exists {
			continue
		}

		// Skip if it's a Docker service (has docker config and no commands)
		if svc.Docker != nil && (svc.Commands == nil || (svc.Commands.Up == "" && svc.Source.Command == "")) {
			continue // Already handled by DockerRunner above
		}

		// This is a host service - check its status
		info := uc.getHostServiceInfo(ctx, wsConcrete, name, svc, originalDeps, hostProcesses)
		if info != nil {
			servicesInfo[name] = info
		}
	}

	// Get active workspace for output
	activeWorkspace, err := uc.deps.Workspace.GetActiveWorkspace()
	if err != nil {
		activeWorkspace = "" // Ignore error, just use empty
	}

	// Output format
	if opts.JSON {
		return uc.outputJSON(servicesInfo, disabledServices, stateDeps, activeWorkspace)
	}

	// Output in human-readable format
	return uc.outputHumanReadable(servicesInfo, disabledServices, stateDeps, activeWorkspace)
}

// outputJSON outputs status in JSON format
func (uc *StatusUseCase) outputJSON(servicesInfo map[string]*interfaces.ServiceInfo, disabledServices []string, stateDeps *config.Deps, activeWorkspace string) error {
	jsonData := map[string]any{
		"project": map[string]string{
			"name":    stateDeps.Project.Name,
			"network": stateDeps.Project.Network,
		},
		"services":      servicesInfo,
		"disabled":      disabledServices,
		"disabledCount": len(disabledServices),
	}
	if activeWorkspace != "" {
		jsonData["activeWorkspace"] = activeWorkspace
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(jsonData); err != nil {
		return fmt.Errorf("failed to encode JSON: %w", err)
	}
	return nil
}

// outputHumanReadable outputs status in human-readable format
func (uc *StatusUseCase) outputHumanReadable(servicesInfo map[string]*interfaces.ServiceInfo, disabledServices []string, stateDeps *config.Deps, activeWorkspace string) error {
	// Table output - these are user-facing output, not logs
	output.PrintSectionHeader("Project Status")
	output.PrintKeyValue("Project", stateDeps.Project.Name)
	output.PrintKeyValue("Network", stateDeps.Project.Network)

	// Show active workspace if set
	if activeWorkspace != "" {
		output.PrintKeyValue("Active Workspace", activeWorkspace)
	}

	output.PrintSubsection("Services")
	if len(servicesInfo) == 0 {
		output.PrintEmptyState("services running")
	} else {
		if err := uc.deps.DockerRunner.FormatStatusTable(servicesInfo, false); err != nil {
			return fmt.Errorf("failed to format table: %w", err)
		}
	}

	// Show disabled services if any
	if len(disabledServices) > 0 {
		output.PrintSubsection(fmt.Sprintf("Disabled Services (%d)", len(disabledServices)))
		output.PrintList(disabledServices, 0)
	}

	return nil
}

// getHostServiceInfo gets status information for a host service
func (uc *StatusUseCase) getHostServiceInfo(
	ctx context.Context,
	ws *workspacepkg.Workspace,
	serviceName string,
	svc config.Service,
	deps *config.Deps,
	hostProcesses map[string]*host.ProcessInfo,
) *interfaces.ServiceInfo {
	info := &interfaces.ServiceInfo{
		Status: "stopped",
		Health: "none",
	}

	// Check if we have ProcessInfo for this service
	processInfo, hasProcessInfo := hostProcesses[serviceName]

	// Get ComposePath: from ProcessInfo first, then from config, then auto-detect
	var composePathToCheck string
	if hasProcessInfo && processInfo.ComposePath != "" {
		composePathToCheck = processInfo.ComposePath
	} else if svc.Commands != nil && svc.Commands.ComposePath != "" {
		// Try to get from config if not in ProcessInfo
		servicePath := workspacepkg.GetServicePath(ws, serviceName, svc)
		if filepath.IsAbs(svc.Commands.ComposePath) {
			composePathToCheck = svc.Commands.ComposePath
		} else {
			composePathToCheck = filepath.Join(servicePath, svc.Commands.ComposePath)
		}
	} else {
		// Try auto-detect composePath
		servicePath := workspacepkg.GetServicePath(ws, serviceName, svc)
		command := ""
		if svc.Source.Command != "" {
			command = svc.Source.Command
		} else if svc.Commands != nil && svc.Commands.Up != "" {
			command = svc.Commands.Up
		}
		explicitComposePath := ""
		if svc.Commands != nil {
			explicitComposePath = svc.Commands.ComposePath
		}
		composePathToCheck = host.DetectComposePath(servicePath, command, explicitComposePath)
	}

	// Priority 1: Check docker-compose if service has ComposePath
	if composePathToCheck != "" {
		// Try to get status from docker-compose
		composeStatus, err := docker.GetServicesStatusWithContext(ctx, composePathToCheck)
		if err == nil && len(composeStatus) > 0 {
			// Look for any running service in that compose file
			// We can't match by service name since compose file might have different service names
			// So if any service is running, consider it as running
			hasRunning := false
			for _, status := range composeStatus {
				if status == "running" {
					hasRunning = true
					info.Status = "running"
					info.Health = "unknown"
					break
				}
			}
			// If compose file exists but no services running, status remains "stopped"
			if !hasRunning {
				info.Status = "stopped"
			}
		}
	}

	// Priority 2: Check health command if available
	if info.Status == "stopped" || info.Health == "none" {
		mode := "dev"
		if svc.Docker != nil && svc.Docker.Mode != "" {
			mode = svc.Docker.Mode
		}

		healthCommand := ""
		if svc.Commands != nil {
			if mode == "prod" && svc.Commands.Prod != nil && svc.Commands.Prod.Health != "" {
				healthCommand = svc.Commands.Prod.Health
			} else if mode == "dev" && svc.Commands.Dev != nil && svc.Commands.Dev.Health != "" {
				healthCommand = svc.Commands.Dev.Health
			} else if svc.Commands.Health != "" {
				healthCommand = svc.Commands.Health
			}
		}

		if healthCommand != "" {
			servicePath := workspacepkg.GetServicePath(ws, serviceName, svc)
			// Execute health command
			cmdParts := strings.Fields(healthCommand)
			if len(cmdParts) > 0 {
				var cmd *exec.Cmd
				if len(cmdParts) == 1 {
					cmd = exec.CommandContext(ctx, cmdParts[0])
				} else {
					cmd = exec.CommandContext(ctx, cmdParts[0], cmdParts[1:]...)
				}
				cmd.Dir = servicePath

				if output, err := cmd.CombinedOutput(); err == nil {
					outputStr := strings.TrimSpace(string(output))
					// Parse health output (on/off or JSON with status)
					isHealthy := parseHealthCommandOutput(outputStr)
					if isHealthy {
						info.Status = "running"
						info.Health = "healthy"
					} else {
						info.Status = "stopped"
						info.Health = "unhealthy"
					}
				}
			}
		}
	}

	// Priority 3: Check ProcessInfo PID (for background processes)
	if hasProcessInfo && processInfo.PID > 0 {
		if running, err := host.IsServiceRunning(processInfo.PID); err == nil && running {
			if info.Status == "stopped" {
				info.Status = "running"
			}
			if info.Health == "none" {
				info.Health = "unknown"
			}
			// Calculate uptime from StartTime
			if !processInfo.StartTime.IsZero() {
				uptime := time.Since(processInfo.StartTime)
				info.Uptime = formatUptimeForStatus(uptime)
			}
		}
	}

	return info
}

// parseHealthCommandOutput parses health command output (same logic as in upcase)
func parseHealthCommandOutput(output string) bool {
	output = strings.TrimSpace(output)
	outputLower := strings.ToLower(output)

	// Check for "on" or "off"
	if outputLower == "on" {
		return true
	}
	if outputLower == "off" {
		return false
	}

	// Try to parse as JSON
	var jsonData map[string]interface{}
	if err := json.Unmarshal([]byte(output), &jsonData); err == nil {
		if status, ok := jsonData["status"].(string); ok {
			statusLower := strings.ToLower(status)
			if statusLower == "active" || statusLower == "running" || statusLower == "healthy" ||
				statusLower == "up" || statusLower == "on" {
				return true
			}
			if statusLower == "inactive" || statusLower == "stopped" || statusLower == "unhealthy" ||
				statusLower == "down" || statusLower == "off" {
				return false
			}
		}
		return true // JSON without status field defaults to healthy
	}

	// Default: any output with exit code 0 is healthy
	return true
}

// formatUptimeForStatus formats duration for status display
func formatUptimeForStatus(d time.Duration) string {
	days := int(d.Hours() / 24)
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}
