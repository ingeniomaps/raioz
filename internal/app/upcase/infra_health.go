package upcase

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"raioz/internal/config"
	"raioz/internal/logging"
	"raioz/internal/naming"
	"raioz/internal/output"
	"raioz/internal/runtime"
)

// checkInfraHealth waits for infrastructure containers to be healthy.
// If a container is restarting or exited, shows the last log lines with actionable suggestions.
// Uses a single `docker inspect` call per cycle for all containers instead of one per container.
func checkInfraHealth(
	ctx context.Context, infraNames []string, projectName string,
	infra map[string]config.InfraEntry,
) error {
	if len(infraNames) == 0 {
		return nil
	}

	// Wait a moment for containers to stabilize (catch fast crash loops)
	time.Sleep(2 * time.Second)

	maxWait := 10 * time.Second
	checkInterval := 1 * time.Second
	deadline := time.Now().Add(maxWait)

	// Build container names once, honoring dep-level name overrides and
	// workspace-shared naming so the health check inspects the exact same
	// containers ImageRunner created.
	containerNames := make([]string, len(infraNames))
	nameIndex := make(map[string]string) // containerName → infraName
	for i, name := range infraNames {
		var nameOverride string
		if entry, ok := infra[name]; ok && entry.Inline != nil {
			nameOverride = entry.Inline.Name
		}
		cn := naming.DepContainer(projectName, name, nameOverride)
		containerNames[i] = cn
		nameIndex[cn] = name
	}

	for time.Now().Before(deadline) {
		// Single docker inspect call for ALL containers
		statuses := getBatchContainerStatuses(ctx, containerNames)

		allHealthy := true
		for _, cn := range containerNames {
			status := statuses[cn]
			infraName := nameIndex[cn]

			switch {
			case status == "running":
				continue
			case status == "restarting" || status == "exited":
				output.PrintWarning(fmt.Sprintf("%s is %s — checking logs...", infraName, status))
				showContainerDiagnostics(ctx, cn, infraName)
				return fmt.Errorf("dependency '%s' failed to start (status: %s)", infraName, status)
			default:
				allHealthy = false
			}
		}

		if allHealthy {
			return nil
		}

		time.Sleep(checkInterval)
	}

	return nil // Timeout but don't block — services might still come up
}

// getBatchContainerStatuses inspects multiple containers in a single docker
// call and returns a map of containerName → status. This replaces N individual
// `docker inspect` calls with one.
func getBatchContainerStatuses(ctx context.Context, containerNames []string) map[string]string {
	result := make(map[string]string, len(containerNames))
	for _, cn := range containerNames {
		result[cn] = "unknown"
	}

	if len(containerNames) == 0 {
		return result
	}

	// docker inspect accepts multiple container names and returns a JSON array
	args := append([]string{"inspect", "--format",
		"{{.Name}}\t{{.State.Status}}"}, containerNames...)
	cmd := exec.CommandContext(ctx, runtime.Binary(), args...)
	out, err := cmd.Output()
	if err != nil {
		return result
	}

	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			continue
		}
		name := strings.TrimPrefix(strings.TrimSpace(parts[0]), "/")
		status := strings.TrimSpace(parts[1])
		result[name] = status
	}

	return result
}

// showContainerDiagnostics shows the last few log lines and actionable suggestions.
func showContainerDiagnostics(ctx context.Context, containerName, serviceName string) {
	// Get last 10 lines of logs
	cmd := exec.CommandContext(ctx, runtime.Binary(), "logs", "--tail", "10", containerName)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return
	}

	logs := strings.TrimSpace(string(out))
	if logs == "" {
		return
	}

	// Show logs indented
	fmt.Println()
	for _, line := range strings.Split(logs, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			fmt.Printf("    %s\n", line)
		}
	}
	fmt.Println()

	// Detect common errors and suggest fixes
	suggestions := diagnoseContainerError(logs, serviceName)
	for _, s := range suggestions {
		output.PrintInfo(s)
	}
}

// diagnoseContainerError inspects error logs and returns actionable suggestions.
func diagnoseContainerError(logs, serviceName string) []string {
	lower := strings.ToLower(logs)
	var suggestions []string

	// Postgres: missing password
	if strings.Contains(lower, "postgres_password") ||
		strings.Contains(lower, "superuser password is not specified") {
		suggestions = append(suggestions,
			fmt.Sprintf("Add env file to '%s' in raioz.yaml:", serviceName),
			fmt.Sprintf("  dependencies:"),
			fmt.Sprintf("    %s:", serviceName),
			fmt.Sprintf("      env: .env.%s", serviceName),
			fmt.Sprintf("Then create .env.%s with: POSTGRES_PASSWORD=postgres", serviceName),
		)
	}

	// MySQL: missing root password
	if strings.Contains(lower, "mysql_root_password") ||
		strings.Contains(lower, "root password") {
		suggestions = append(suggestions,
			fmt.Sprintf("Create .env.%s with: MYSQL_ROOT_PASSWORD=root", serviceName),
			fmt.Sprintf("Add env: .env.%s to '%s' in raioz.yaml", serviceName, serviceName),
		)
	}

	// Permission denied
	if strings.Contains(lower, "permission denied") {
		suggestions = append(suggestions,
			"Check volume mount permissions",
			"Try: docker volume rm and recreate",
		)
	}

	// Port already in use
	if strings.Contains(lower, "address already in use") ||
		strings.Contains(lower, "bind: address already in use") {
		suggestions = append(suggestions,
			fmt.Sprintf("Port is already in use by another process"),
			"Run: raioz down && raioz up to restart cleanly",
		)
	}

	// Generic: no specific diagnosis
	if len(suggestions) == 0 {
		suggestions = append(suggestions,
			fmt.Sprintf("Check full logs: raioz logs %s", serviceName),
		)
	}

	return suggestions
}

// checkHostServiceHealth verifies host services started successfully.
func checkHostServiceHealth(ctx context.Context, serviceName, logPath string) {
	// Read log file for errors
	cmd := exec.CommandContext(ctx, "tail", "-n", "5", logPath)
	out, err := cmd.Output()
	if err != nil {
		return
	}

	logs := strings.TrimSpace(string(out))
	lower := strings.ToLower(logs)

	// Check for common host service errors
	if strings.Contains(lower, "address already in use") {
		output.PrintWarning(fmt.Sprintf("%s: port already in use", serviceName))
		output.PrintInfo("A previous instance may still be running. Try: raioz down && raioz up")
	} else if strings.Contains(lower, "error") || strings.Contains(lower, "fatal") ||
		strings.Contains(lower, "panic") {
		logging.Warn("Host service may have errors", "service", serviceName)
	}
}
