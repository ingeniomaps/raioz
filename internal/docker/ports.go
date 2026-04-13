package docker

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"raioz/internal/config"
)

// PortInfo contains information about a port usage
type PortInfo struct {
	Port          string
	Project       string
	Service       string
	HostPort      int
	ContainerPort int
}

// CheckPortInUse checks if a host port is in use by trying to bind to it
func CheckPortInUse(port string) (bool, error) {
	// Parse port (format: "host:container" or just "host")
	parts := strings.Split(port, ":")
	if len(parts) == 0 || parts[0] == "" {
		return false, fmt.Errorf("invalid port format: %s", port)
	}

	hostPortStr := parts[0]
	hostPort, err := strconv.Atoi(hostPortStr)
	if err != nil {
		return false, fmt.Errorf("invalid port number: %s", hostPortStr)
	}

	// Try to bind to the port
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", hostPort))
	if err != nil {
		// Port is in use
		return true, nil
	}

	// Port is available, close the listener
	ln.Close()
	return false, nil
}

// ParsePort extracts host port from port string (format: "host:container")
func ParsePort(port string) (int, error) {
	parts := strings.Split(port, ":")
	if len(parts) == 0 || parts[0] == "" {
		return 0, fmt.Errorf("invalid port format: %s", port)
	}

	hostPort, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, fmt.Errorf("invalid port number: %s", parts[0])
	}

	return hostPort, nil
}

// GetAllActivePorts scans all workspace state files to find active ports
func GetAllActivePorts(baseDir string) ([]PortInfo, error) {
	var ports []PortInfo
	workspacesDir := filepath.Join(baseDir, "workspaces")

	// Read workspaces directory
	entries, err := os.ReadDir(workspacesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return ports, nil // No workspaces yet
		}
		return nil, fmt.Errorf("failed to read workspaces: %w", err)
	}

	// Check each workspace for state file
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		projectName := entry.Name()
		statePath := filepath.Join(workspacesDir, projectName, ".state.json")

		// Try to load state
		data, err := os.ReadFile(statePath)
		if err != nil {
			continue // Skip if can't read
		}

		// Parse JSON to extract ports
		var state struct {
			Project struct {
				Name string `json:"name"`
			} `json:"project"`
			Services map[string]struct {
				Docker struct {
					Ports []string `json:"ports"`
				} `json:"docker"`
			} `json:"services"`
			Infra map[string]struct {
				Ports []string `json:"ports"`
			} `json:"infra"`
		}

		if err := json.Unmarshal(data, &state); err != nil {
			continue // Skip if invalid JSON
		}

		// Extract ports from services
		for serviceName, svc := range state.Services {
			// Skip if no ports (host execution or no docker config)
			if len(svc.Docker.Ports) == 0 {
				continue
			}
			for _, port := range svc.Docker.Ports {
				hostPort, err := ParsePort(port)
				if err != nil {
					continue // Skip invalid ports
				}

				containerPort := hostPort
				parts := strings.Split(port, ":")
				if len(parts) == 2 {
					if cp, err := strconv.Atoi(parts[1]); err == nil {
						containerPort = cp
					}
				}

				ports = append(ports, PortInfo{
					Port:          port,
					Project:       projectName,
					Service:       serviceName,
					HostPort:      hostPort,
					ContainerPort: containerPort,
				})
			}
		}

		// Extract ports from infra
		for infraName, infra := range state.Infra {
			for _, port := range infra.Ports {
				hostPort, err := ParsePort(port)
				if err != nil {
					continue // Skip invalid ports
				}

				containerPort := hostPort
				parts := strings.Split(port, ":")
				if len(parts) == 2 {
					if cp, err := strconv.Atoi(parts[1]); err == nil {
						containerPort = cp
					}
				}

				ports = append(ports, PortInfo{
					Port:          port,
					Project:       projectName,
					Service:       infraName,
					HostPort:      hostPort,
					ContainerPort: containerPort,
				})
			}
		}
	}

	return ports, nil
}

// GetProjectUsingPort finds which project is using a specific host port
func GetProjectUsingPort(port string, baseDir string) (*PortInfo, error) {
	hostPort, err := ParsePort(port)
	if err != nil {
		return nil, err
	}

	activePorts, err := GetAllActivePorts(baseDir)
	if err != nil {
		return nil, err
	}

	// Find port in active ports
	for _, portInfo := range activePorts {
		if portInfo.HostPort == hostPort {
			return &portInfo, nil
		}
	}

	return nil, nil // Port not found
}

// FindAlternativePort suggests an alternative port if the requested one is in use
func FindAlternativePort(port string, maxAttempts int) (int, error) {
	hostPort, err := ParsePort(port)
	if err != nil {
		return 0, err
	}

	// Try nearby ports
	for offset := 1; offset <= maxAttempts; offset++ {
		// Try port + offset
		candidate := hostPort + offset
		inUse, err := CheckPortInUse(fmt.Sprintf("%d", candidate))
		if err != nil {
			continue
		}
		if !inUse {
			return candidate, nil
		}
	}

	return 0, fmt.Errorf("no available alternative port found near %d", hostPort)
}

// ValidatePorts checks if all ports in a project are available
func ValidatePorts(deps *config.Deps, baseDir string, projectName string) ([]PortConflict, error) {
	var conflicts []PortConflict
	var allPorts []string

	// Extract ports from services
	for _, svc := range deps.Services {
		// Skip if docker is nil (host execution - no docker ports)
		if svc.Docker == nil {
			continue
		}
		allPorts = append(allPorts, svc.Docker.Ports...)
	}

	// Extract ports from inline infra
	for _, entry := range deps.Infra {
		if entry.Inline != nil {
			allPorts = append(allPorts, entry.Inline.Ports...)
		}
	}

	// Check each port
	for _, port := range allPorts {
		// Check if port is in use
		inUse, err := CheckPortInUse(port)
		if err != nil {
			continue // Skip invalid ports
		}

		if inUse {
			// Check if it's used by another project
			portInfo, err := GetProjectUsingPort(port, baseDir)
			if err == nil && portInfo != nil && portInfo.Project != projectName {
				// Try to find alternative port
				altPort, err := FindAlternativePort(port, 10)
				altStr := ""
				if err == nil && altPort > 0 {
					altStr = fmt.Sprintf("%d", altPort)
				}

				conflicts = append(conflicts, PortConflict{
					Port:        port,
					Project:     portInfo.Project,
					Service:     portInfo.Service,
					Alternative: altStr,
				})
			}
		}
	}

	return conflicts, nil
}

// PortConflict represents a port conflict with another project
type PortConflict struct {
	Port        string
	Project     string
	Service     string
	Alternative string
}

// FormatPortConflicts formats port conflicts for display
func FormatPortConflicts(conflicts []PortConflict) string {
	if len(conflicts) == 0 {
		return ""
	}

	var lines []string
	for _, conflict := range conflicts {
		line := fmt.Sprintf(
			"  Port %s is used by project '%s', service '%s'",
			conflict.Port, conflict.Project, conflict.Service,
		)
		if conflict.Alternative != "" {
			line += fmt.Sprintf(" (suggested alternative: %s)", conflict.Alternative)
		}
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}
