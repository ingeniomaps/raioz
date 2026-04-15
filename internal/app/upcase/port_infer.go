package upcase

import (
	"os"
	"path/filepath"
	"strings"

	"raioz/internal/config"
	"raioz/internal/detect"
)

// inferServicePort tries to determine the port a host service listens on.
// It checks: config ports, .env PORT variable, then runtime defaults.
func inferServicePort(svc config.Service, detection detect.DetectResult) int {
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
	case detect.RuntimeGo:
		return 8080
	case detect.RuntimeNPM:
		return 3000
	case detect.RuntimePython:
		return 5000
	case detect.RuntimeRust:
		return 8080
	case detect.RuntimePHP:
		return 8000
	case detect.RuntimeJava, detect.RuntimeScala, detect.RuntimeClojure:
		return 8080
	case detect.RuntimeDotnet:
		return 5000
	case detect.RuntimeRuby:
		return 3000
	case detect.RuntimeElixir:
		return 4000
	case detect.RuntimeDart:
		return 8080
	case detect.RuntimeSwift:
		return 8080
	case detect.RuntimeZig:
		return 8080
	case detect.RuntimeGleam:
		return 8080
	case detect.RuntimeHaskell:
		return 3000
	case detect.RuntimeDeno, detect.RuntimeBun:
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
