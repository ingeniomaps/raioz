package docker

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"raioz/internal/config"
	"raioz/internal/workspace"
	pathvalidate "raioz/internal/path"
)

// ValidateDockerfile checks if Dockerfile.dev exists for a git service
func ValidateDockerfile(servicePath string, dockerfile string) (bool, error) {
	// Validate path to prevent path traversal
	dockerfilePath, err := pathvalidate.EnsurePathInBase(servicePath, dockerfile)
	if err != nil {
		return false, fmt.Errorf("invalid dockerfile path: %w", err)
	}

	_, err = os.Stat(dockerfilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to check dockerfile: %w", err)
	}
	return true, nil
}

// GenerateDockerfileWrapper generates a temporary Dockerfile wrapper
// when Dockerfile.dev doesn't exist but command is specified
func GenerateDockerfileWrapper(
	ws *workspace.Workspace,
	serviceName string,
	svc config.Service,
) (string, error) {
	// Determine base image based on runtime or default
	baseImage := getBaseImageForRuntime(svc.Docker.Runtime)

	// Create wrapper Dockerfile
	installCmd := getInstallCommand(svc.Docker.Runtime)
	runCommand := svc.Docker.Command

	// For Node.js in dev mode, prepend npm install to ensure dependencies are available
	// This is needed because dev volumes override the node_modules from the build
	// Only prepend if the command doesn't already include npm install
	if svc.Docker.Mode == "dev" && (svc.Docker.Runtime == "node" || svc.Docker.Runtime == "nodejs" || svc.Docker.Runtime == "javascript" || svc.Docker.Runtime == "js" || svc.Docker.Runtime == "") {
		// Check if command already includes npm install
		commandLower := strings.ToLower(svc.Docker.Command)
		if !strings.Contains(commandLower, "npm install") && !strings.Contains(commandLower, "npm i ") {
			// Prepend npm install to the command to ensure dependencies are installed
			// Use sh -c to run multiple commands
			installShellCmd := "if [ -f package.json ]; then npm install --legacy-peer-deps || npm install --force --legacy-peer-deps || npm install --force || true; fi"
			runCommand = fmt.Sprintf("sh -c '%s && %s'", installShellCmd, svc.Docker.Command)
		}
	}

	dockerfileContent := fmt.Sprintf(`FROM %s

WORKDIR /app

# Copy project files
COPY . .

# Install dependencies (if package.json, go.mod, requirements.txt exist)
%s

# Run the command
CMD %s
`, baseImage, installCmd, runCommand)

	// Write to temporary location in workspace
	wrapperPath := filepath.Join(ws.Root, fmt.Sprintf("Dockerfile.%s.wrapper", serviceName))
	if err := os.WriteFile(wrapperPath, []byte(dockerfileContent), 0644); err != nil {
		return "", fmt.Errorf("failed to write wrapper dockerfile: %w", err)
	}

	return wrapperPath, nil
}

// getBaseImageForRuntime returns the base Docker image for a given runtime
func getBaseImageForRuntime(runtime string) string {
	switch strings.ToLower(runtime) {
	case "node", "nodejs", "javascript", "js":
		return "node:22-alpine"
	case "go", "golang":
		return "golang:1.22-alpine"
	case "python", "py":
		return "python:3.11-alpine"
	case "java":
		return "openjdk:17-alpine"
	case "rust":
		return "rust:1.75-alpine"
	default:
		// Default to node for backward compatibility
		return "node:22-alpine"
	}
}

// getInstallCommand returns the install command for a given runtime
func getInstallCommand(runtime string) string {
	switch strings.ToLower(runtime) {
	case "node", "nodejs", "javascript", "js":
		// Use --legacy-peer-deps to handle peer dependency conflicts gracefully
		// Fallback to --force if workspace protocol is not supported
		// Final fallback to regular npm install (may fail with dependency conflicts)
		return `RUN if [ -f package.json ]; then npm install --legacy-peer-deps || npm install --force --legacy-peer-deps || npm install --force || true; fi`
	case "go", "golang":
		return `RUN if [ -f go.mod ]; then go mod download; fi`
	case "python", "py":
		return `RUN if [ -f requirements.txt ]; then pip install -r requirements.txt; fi`
	case "java":
		return `RUN if [ -f pom.xml ]; then mvn dependency:go-offline || true; fi`
	case "rust":
		return `RUN if [ -f Cargo.toml ]; then cargo fetch || true; fi`
	default:
		return `RUN if [ -f package.json ]; then npm install; fi`
	}
}

// EnsureDockerfile ensures that a Dockerfile exists for a git service
// Returns the path to the dockerfile (either existing or generated wrapper)
func EnsureDockerfile(
	ws *workspace.Workspace,
	serviceName string,
	svc config.Service,
) (string, error) {
	if svc.Source.Kind != "git" {
		return "", nil // Not applicable for image services
	}

	// Check if Docker config exists
	if svc.Docker == nil {
		return "", fmt.Errorf("service %s requires docker configuration", serviceName)
	}

	servicePath := workspace.GetServicePath(ws, serviceName, svc)
	dockerfile := svc.Docker.Dockerfile

	// If dockerfile is not specified, default to "Dockerfile.dev"
	if dockerfile == "" {
		dockerfile = "Dockerfile.dev"
	}

	// Check if dockerfile exists
	exists, err := ValidateDockerfile(servicePath, dockerfile)
	if err != nil {
		return "", fmt.Errorf("failed to validate dockerfile: %w", err)
	}

	if exists {
		// Dockerfile exists, return path relative to service
		return dockerfile, nil
	}

	// Dockerfile doesn't exist, check if we can generate wrapper
	if svc.Docker.Command == "" {
		return "", fmt.Errorf(
			"Dockerfile.dev not found in %s (path: %s). "+
				"Add Dockerfile.dev to the repo or specify 'command' in docker config",
			serviceName, servicePath,
		)
	}

	// Generate wrapper
	wrapperPath, err := GenerateDockerfileWrapper(ws, serviceName, svc)
	if err != nil {
		return "", fmt.Errorf("failed to generate dockerfile wrapper: %w", err)
	}

	// Return absolute path to wrapper (will be used as dockerfile in compose)
	return wrapperPath, nil
}
