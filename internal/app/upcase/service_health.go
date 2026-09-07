package upcase

import (
	"encoding/json"
	"strings"
)

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
