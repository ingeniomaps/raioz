package upcase

import (
	"os"
	"path/filepath"
	"strings"

	"raioz/internal/domain/models"
)

// inferServicePort tries to determine the port a host service listens on.
// It checks: config ports, .env PORT variable, then runtime defaults.
func inferServicePort(svc models.Service, detection models.DetectResult) int {
	// 1. Config ports (e.g., ports: ["3000"])
	if svc.Docker != nil && len(svc.Docker.Ports) > 0 {
		if p := parseFirstPort(svc.Docker.Ports[0]); p > 0 {
			return p
		}
	}

	// 2. Read PORT from service's .env file
	if svc.Source.Path != "" {
		envPath := filepath.Join(svc.Source.Path, ".env")
		if data, err := os.ReadFile(envPath); err == nil {
			for _, line := range strings.Split(string(data), "\n") {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "PORT=") {
					if p := parseFirstPort(strings.TrimPrefix(line, "PORT=")); p > 0 {
						return p
					}
				}
			}
		}
	}

	// 3. Runtime defaults
	switch detection.Runtime {
	case models.RuntimeGo:
		return 8080
	case models.RuntimeNPM:
		return 3000
	case models.RuntimePython:
		return 5000
	case models.RuntimeRust:
		return 8080
	case models.RuntimePHP:
		return 8000
	case models.RuntimeJava, models.RuntimeScala, models.RuntimeClojure:
		return 8080
	case models.RuntimeDotnet:
		return 5000
	case models.RuntimeRuby:
		return 3000
	case models.RuntimeElixir:
		return 4000
	case models.RuntimeDart:
		return 8080
	case models.RuntimeSwift:
		return 8080
	case models.RuntimeZig:
		return 8080
	case models.RuntimeGleam:
		return 8080
	case models.RuntimeHaskell:
		return 3000
	case models.RuntimeDeno, models.RuntimeBun:
		return 3000
	}

	return 0
}

// parseFirstPort extracts the host port from a port mapping like "8080:3000" or "5432".
func parseFirstPort(portSpec string) int {
	parts := strings.SplitN(portSpec, ":", 2)
	portStr := parts[0]

	port := 0
	for _, ch := range portStr {
		if ch >= '0' && ch <= '9' {
			port = port*10 + int(ch-'0')
		} else {
			break
		}
	}
	return port
}
