package production

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// LoadComposeFile loads a Docker Compose file from the given path
func LoadComposeFile(path string) (*ProductionConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read compose file: %w", err)
	}

	var config ProductionConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse compose file: %w", err)
	}

	if config.Services == nil {
		config.Services = make(map[string]ProductionService)
	}

	return &config, nil
}

// NormalizePorts converts docker-compose port formats to standard format
func NormalizePorts(ports []string) []string {
	if ports == nil {
		return []string{}
	}

	normalized := make([]string, 0, len(ports))
	for _, port := range ports {
		// Handle formats like "3000:3000", "3000:3000/tcp", "127.0.0.1:3000:3000"
		parts := strings.Split(port, ":")
		if len(parts) >= 2 {
			// Extract host:container port mapping
			hostPort := parts[len(parts)-2]
			containerPort := parts[len(parts)-1]
			// Remove protocol if present (e.g., "/tcp")
			containerPort = strings.Split(containerPort, "/")[0]
			normalized = append(normalized, fmt.Sprintf("%s:%s", hostPort, containerPort))
		} else {
			normalized = append(normalized, port)
		}
	}

	return normalized
}

// ParseDependsOn converts depends_on from various formats to a list of service names
func ParseDependsOn(dependsOn interface{}) []string {
	if dependsOn == nil {
		return []string{}
	}

	var deps []string

	switch v := dependsOn.(type) {
	case []interface{}:
		for _, dep := range v {
			switch d := dep.(type) {
			case string:
				deps = append(deps, d)
			case map[string]interface{}:
				// Handle format: depends_on: [service: {condition: service_healthy}]
				for serviceName := range d {
					deps = append(deps, serviceName)
				}
			}
		}
	case []string:
		deps = v
	case map[string]interface{}:
		// Handle format: depends_on: {service: {condition: service_healthy}}
		for serviceName := range v {
			deps = append(deps, serviceName)
		}
	}

	return deps
}

// ExtractImageAndTag extracts image name and tag from a full image string
func ExtractImageAndTag(imageStr string) (image, tag string) {
	if imageStr == "" {
		return "", ""
	}

	// Handle format: "image:tag" or "registry/image:tag"
	parts := strings.Split(imageStr, ":")
	if len(parts) >= 2 {
		tag = parts[len(parts)-1]
		image = strings.Join(parts[:len(parts)-1], ":")
	} else {
		image = imageStr
		tag = "latest"
	}

	return image, tag
}

// IsServiceName checks if a name matches a service in the production config
func (pc *ProductionConfig) IsServiceName(name string) bool {
	_, exists := pc.Services[name]
	return exists
}

// GetServiceNames returns all service names from the production config
func (pc *ProductionConfig) GetServiceNames() []string {
	names := make([]string, 0, len(pc.Services))
	for name := range pc.Services {
		names = append(names, name)
	}
	return names
}

// ResolveAbsolutePath resolves a path relative to the compose file's directory
func ResolveAbsolutePath(composeFilePath, targetPath string) string {
	if filepath.IsAbs(targetPath) {
		return targetPath
	}

	composeDir := filepath.Dir(composeFilePath)
	return filepath.Join(composeDir, targetPath)
}
