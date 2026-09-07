package docker

import (
	"fmt"
	"os"
)

// AreServicesRunning checks if all required services are already running
func AreServicesRunning(composePath string, serviceNames []string) (bool, error) {
	if _, err := os.Stat(composePath); os.IsNotExist(err) {
		return false, nil
	}

	statuses, err := GetServicesStatus(composePath)
	if err != nil {
		return false, fmt.Errorf("failed to get services status: %w", err)
	}

	// Check if all services are running
	for _, name := range serviceNames {
		if status, ok := statuses[name]; !ok || status != "running" {
			return false, nil
		}
	}

	return true, nil
}
