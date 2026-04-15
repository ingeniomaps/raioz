package detect

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// inferFromCompose extracts useful info from a compose project directory.
func inferFromCompose(result *DetectResult, path string) {
	// Also check if there's a Dockerfile alongside (for builds)
	dockerfile := filepath.Join(path, "Dockerfile")
	if fileExists(dockerfile) {
		result.Dockerfile = dockerfile
		result.Files = append(result.Files, "Dockerfile")
	}

	result.StartCommand = "docker compose up -d"
	result.DevCommand = "docker compose up"
	result.HasHotReload = false
}

// inferFromDockerfile extracts useful info when only a Dockerfile is present.
func inferFromDockerfile(result *DetectResult, path string) {
	result.StartCommand = "docker build -t service . && docker run service"

	// Check if there's also a package.json or go.mod for better port inference
	if fileExists(filepath.Join(path, "package.json")) {
		result.Files = append(result.Files, "package.json")
		inferPortFromPackageJSON(result, filepath.Join(path, "package.json"))
	} else if fileExists(filepath.Join(path, "go.mod")) {
		result.Files = append(result.Files, "go.mod")
	}
}

// packageJSON is a minimal representation for parsing.
type packageJSON struct {
	Scripts map[string]string `json:"scripts"`
	Main    string            `json:"main"`
}

// inferFromPackageJSON parses package.json to find dev command and port.
func inferFromPackageJSON(result *DetectResult, path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		result.StartCommand = "npm start"
		return
	}

	var pkg packageJSON
	if err := json.Unmarshal(data, &pkg); err != nil {
		result.StartCommand = "npm start"
		return
	}

	// Prefer dev script, fall back to start
	if cmd, ok := pkg.Scripts["dev"]; ok {
		result.DevCommand = "npm run dev"
		result.StartCommand = "npm run dev"
		result.HasHotReload = hasHotReload(cmd)
		inferPortFromScript(result, cmd)
	} else if _, ok := pkg.Scripts["start"]; ok {
		result.StartCommand = "npm start"
		result.DevCommand = "npm start"
	} else {
		result.StartCommand = "npm start"
	}

	// Check for common frameworks that have hot-reload
	if !result.HasHotReload {
		for _, script := range pkg.Scripts {
			if hasHotReload(script) {
				result.HasHotReload = true
				break
			}
		}
	}

	inferPortFromPackageJSON(result, path)
}

// hasHotReload checks if a script command indicates built-in hot-reload.
func hasHotReload(cmd string) bool {
	hotReloadIndicators := []string{
		"next dev", "vite", "nuxt dev", "remix dev",
		"nodemon", "ts-node-dev", "tsx watch",
		"webpack serve", "webpack-dev-server",
		"ng serve", "react-scripts start",
	}
	cmdLower := strings.ToLower(cmd)
	for _, indicator := range hotReloadIndicators {
		if strings.Contains(cmdLower, indicator) {
			return true
		}
	}
	return false
}

var portRegex = regexp.MustCompile(`(?:--port|PORT=|-p\s+)\s*(\d{4,5})`)

// inferPortFromScript tries to extract a port from a script command.
func inferPortFromScript(result *DetectResult, cmd string) {
	if result.Port > 0 {
		return
	}
	matches := portRegex.FindStringSubmatch(cmd)
	if len(matches) > 1 {
		port := 0
		for _, ch := range matches[1] {
			port = port*10 + int(ch-'0')
		}
		if port > 0 && port < 65536 {
			result.Port = port
		}
	}
}

// inferPortFromPackageJSON tries to find port in package.json scripts.
func inferPortFromPackageJSON(result *DetectResult, path string) {
	if result.Port > 0 {
		return
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return
	}

	var pkg packageJSON
	if err := json.Unmarshal(data, &pkg); err != nil {
		return
	}

	// Check all scripts for port patterns
	for _, script := range pkg.Scripts {
		inferPortFromScript(result, script)
		if result.Port > 0 {
			return
		}
	}

	// Default ports for known frameworks
	for _, script := range pkg.Scripts {
		lower := strings.ToLower(script)
		switch {
		case strings.Contains(lower, "next"):
			result.Port = 3000
			return
		case strings.Contains(lower, "vite"):
			result.Port = 5173
			return
		case strings.Contains(lower, "nuxt"):
			result.Port = 3000
			return
		case strings.Contains(lower, "react-scripts"):
			result.Port = 3000
			return
		case strings.Contains(lower, "angular") || strings.Contains(lower, "ng serve"):
			result.Port = 4200
			return
		}
	}
}

// inferFromMakefile checks for common targets in a Makefile.
func inferFromMakefile(result *DetectResult, path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		result.StartCommand = "make"
		return
	}

	content := string(data)
	targets := []string{"dev", "run", "start", "serve", "up"}

	for _, target := range targets {
		if strings.Contains(content, target+":") {
			result.StartCommand = "make " + target
			result.DevCommand = "make " + target
			return
		}
	}

	result.StartCommand = "make"
}
