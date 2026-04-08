package upcase

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	"raioz/internal/config"
	"raioz/internal/domain/interfaces"
)

// getServiceHealthCommand gets the health command for a service
func getServiceHealthCommand(svc config.Service, mode string) string {
	if svc.Commands == nil {
		return ""
	}

	if mode == "" {
		mode = "dev"
	}

	if mode == "prod" && svc.Commands.Prod != nil && svc.Commands.Prod.Health != "" {
		return svc.Commands.Prod.Health
	}
	if mode == "dev" && svc.Commands.Dev != nil && svc.Commands.Dev.Health != "" {
		return svc.Commands.Dev.Health
	}
	if svc.Commands.Health != "" {
		return svc.Commands.Health
	}

	return ""
}

// checkServiceHealthDefault checks service health using default method (process/port)
func checkServiceHealthDefault(ctx context.Context, ws *interfaces.Workspace, serviceName string, svc config.Service) (bool, error) {
	// Try to check if process is running by checking PID file or process list
	// For now, we'll check if there's a port exposed and try to connect to it
	if svc.Docker != nil && len(svc.Docker.Ports) > 0 {
		// Extract port from first port mapping (format: "host:container")
		portMapping := svc.Docker.Ports[0]
		parts := strings.Split(portMapping, ":")
		if len(parts) == 2 {
			hostPort, err := strconv.Atoi(parts[0])
			if err == nil {
				// Try to connect to the port
				conn, err := net.Dial("tcp", fmt.Sprintf("localhost:%d", hostPort))
				if err == nil {
					conn.Close()
					return true, nil // Port is open, service is healthy
				}
				return false, nil // Port is closed, service is not healthy
			}
		}
	}

	// If no port check possible, check if there's a process running
	// This is a simple check - can be enhanced later
	// For now, return false (not healthy) to allow starting
	return false, nil
}

// checkServiceHealth checks if a service is healthy
func checkServiceHealth(ctx context.Context, ws *interfaces.Workspace, serviceName string, svc config.Service, mode string, wm interfaces.WorkspaceManager) (bool, error) {
	// Get health command
	healthCommand := getServiceHealthCommand(svc, mode)

	if healthCommand != "" {
		// Use custom health command
		servicePath := wm.GetServicePath(ws, serviceName, svc)
		cmdParts := strings.Fields(healthCommand)
		if len(cmdParts) == 0 {
			return false, nil
		}

		var cmd *exec.Cmd
		if len(cmdParts) == 1 {
			cmd = exec.CommandContext(ctx, cmdParts[0])
		} else {
			cmd = exec.CommandContext(ctx, cmdParts[0], cmdParts[1:]...)
		}

		cmd.Dir = servicePath
		cmd.Env = os.Environ()

		// Capture stdout to parse response
		output, err := cmd.CombinedOutput()
		outputStr := strings.TrimSpace(string(output))

		if err != nil {
			// Command failed, service is not healthy
			return false, nil
		}

		// Parse output to determine health status
		return parseHealthCommandOutput(outputStr), nil
	}

	// No health command, use default health check
	return checkServiceHealthDefault(ctx, ws, serviceName, svc)
}

// parseHealthCommandOutput parses the output of a health command to determine health status
// Supports multiple formats:
// 1. "on" -> healthy
// 2. "off" -> not healthy
// 3. JSON with status field: {"status":"active"} -> healthy, {"status":"inactive"} -> not healthy
// 4. Docker healthcheck output (any output) -> healthy if exit code 0
// 5. Empty output -> healthy if exit code 0
func parseHealthCommandOutput(output string) bool {
	output = strings.TrimSpace(output)

	// Case 1: "on" or "off" strings
	outputLower := strings.ToLower(output)
	if outputLower == "on" {
		return true
	}
	if outputLower == "off" {
		return false
	}

	// Case 2: Try to parse as JSON
	var jsonData map[string]interface{}
	if err := json.Unmarshal([]byte(output), &jsonData); err == nil {
		// Valid JSON, check for status field
		if status, ok := jsonData["status"].(string); ok {
			statusLower := strings.ToLower(status)
			// Active states: "active", "running", "healthy", "up", "on"
			if statusLower == "active" || statusLower == "running" || statusLower == "healthy" ||
			   statusLower == "up" || statusLower == "on" {
				return true
			}
			// Inactive states: "inactive", "stopped", "unhealthy", "down", "off"
			if statusLower == "inactive" || statusLower == "stopped" || statusLower == "unhealthy" ||
			   statusLower == "down" || statusLower == "off" {
				return false
			}
		}
		// JSON without status field or unknown status -> default to true (command succeeded)
		return true
	}

	// Case 3: Non-JSON output (Docker healthcheck output or any other output)
	// If command succeeded (exit code 0) and produced output, consider it healthy
	// Empty output with exit code 0 is also considered healthy
	return true
}

// isProcessRunning checks if a process with given PID is running
func isProcessRunning(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// Send signal 0 to check if process exists
	err = process.Signal(syscall.Signal(0))
	return err == nil
}
