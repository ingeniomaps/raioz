package upcase

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"raioz/internal/logging"
	"raioz/internal/naming"
	"raioz/internal/output"
)

// checkInfraHealth waits for infrastructure containers to be healthy.
// If a container is restarting or exited, shows the last log lines with actionable suggestions.
func checkInfraHealth(ctx context.Context, infraNames []string, projectName string) error {
	if len(infraNames) == 0 {
		return nil
	}

	// Wait a moment for containers to stabilize (catch fast crash loops)
	time.Sleep(2 * time.Second)

	maxWait := 10 * time.Second
	checkInterval := 1 * time.Second
	deadline := time.Now().Add(maxWait)

	for time.Now().Before(deadline) {
		allHealthy := true
		for _, name := range infraNames {
			containerName := naming.Container(projectName, name)
			status := getContainerStatus(ctx, containerName)

			switch {
			case status == "running":
				continue
			case status == "restarting" || status == "exited":
				// Container is failing — show diagnostics immediately
				output.PrintWarning(fmt.Sprintf("%s is %s — checking logs...", name, status))
				showContainerDiagnostics(ctx, containerName, name)
				return fmt.Errorf("dependency '%s' failed to start (status: %s)", name, status)
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

// getContainerStatus returns the status of a Docker container.
func getContainerStatus(ctx context.Context, containerName string) string {
	cmd := exec.CommandContext(ctx, "docker", "inspect",
		"--format", "{{.State.Status}}", containerName)
	out, err := cmd.Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}

// showContainerDiagnostics shows the last few log lines and actionable suggestions.
func showContainerDiagnostics(ctx context.Context, containerName, serviceName string) {
	// Get last 10 lines of logs
	cmd := exec.CommandContext(ctx, "docker", "logs", "--tail", "10", containerName)
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
