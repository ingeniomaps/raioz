package host

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"raioz/internal/config"
	"raioz/internal/env"
	"raioz/internal/workspace"
)

// resolveEnvVars resolves environment variables for a host service
func resolveEnvVars(ctx context.Context, ws *workspace.Workspace, deps *config.Deps, serviceName string, svc config.Service, projectDir string, servicePath string) ([]string, error) {
	// Resolve env file path (same logic as Docker)
	envFilePath, err := env.ResolveEnvFileForService(ws, deps, serviceName, svc.Env, projectDir, servicePath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve env file: %w", err)
	}

	var envVars []string
	if envFilePath != "" {
		// Read env file and parse into key=value pairs
		data, err := os.ReadFile(envFilePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read env file: %w", err)
		}

		// Simple parsing: split by lines, skip comments and empty lines
		lines := parseEnvFile(string(data))
		envVars = append(envVars, lines...)
	}

	return envVars, nil
}

// parseCommand parses a command string into command and arguments
// Uses shell-like parsing: splits by spaces, handles quoted strings
func parseCommand(cmdStr string) []string {
	if cmdStr == "" {
		return nil
	}

	// For now, use simple split (can be enhanced later for quoted strings)
	// This works for most common cases: "npm run dev", "go run main.go", etc.
	parts := strings.Fields(cmdStr)
	if len(parts) == 0 {
		return nil
	}

	return parts
}

// createVolumeSymlinks creates symbolic links for host services
// volumes format: ["SRC:DEST", ...] where:
// - SRC is relative to projectDir (or absolute path)
// - DEST is relative to servicePath
func createVolumeSymlinks(volumes []string, projectDir string, servicePath string) error {
	for _, vol := range volumes {
		if vol == "" {
			continue
		}

		// Parse SRC:DEST format
		parts := strings.SplitN(vol, ":", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid volume format '%s': expected 'SRC:DEST'", vol)
		}

		src := strings.TrimSpace(parts[0])
		dest := strings.TrimSpace(parts[1])

		if src == "" || dest == "" {
			return fmt.Errorf("invalid volume format '%s': SRC and DEST cannot be empty", vol)
		}

		// Resolve SRC to absolute path (relative to projectDir if not absolute)
		var srcAbs string
		if filepath.IsAbs(src) {
			srcAbs = src
		} else {
			if projectDir == "" {
				return fmt.Errorf("cannot resolve relative path '%s': projectDir is not provided", src)
			}
			srcAbs = filepath.Join(projectDir, src)
		}

		// Create source path if it doesn't exist (file or directory)
		if _, err := os.Stat(srcAbs); os.IsNotExist(err) {
			// Source doesn't exist, determine if it should be a file or directory
			// Check if destination suggests it's a file (has file extension or doesn't end with common dir patterns)
			destBase := filepath.Base(dest)
			hasExtension := filepath.Ext(destBase) != ""
			isLikelyFile := hasExtension && destBase != "." && destBase != ".."

			if isLikelyFile {
				// Likely a file - ensure parent directory exists and create empty file
				parentDir := filepath.Dir(srcAbs)
				if err := os.MkdirAll(parentDir, 0755); err != nil {
					return fmt.Errorf("failed to create parent directory for source file: %w", err)
				}
				// Create empty file
				file, err := os.Create(srcAbs)
				if err != nil {
					return fmt.Errorf("failed to create source file: %w", err)
				}
				file.Close()
			} else {
				// Likely a directory - create directory
				if err := os.MkdirAll(srcAbs, 0755); err != nil {
					return fmt.Errorf("failed to create source directory: %w", err)
				}
			}
		} else if err != nil {
			return fmt.Errorf("failed to check source path: %w", err)
		}

		// Resolve DEST to absolute path (relative to servicePath)
		destAbs := filepath.Join(servicePath, dest)

		// Ensure parent directory of destination exists
		destParent := filepath.Dir(destAbs)
		if err := os.MkdirAll(destParent, 0755); err != nil {
			return fmt.Errorf("failed to create parent directory for destination: %w", err)
		}

		// Check if destination already exists
		if _, err := os.Lstat(destAbs); err == nil {
			// Destination exists, check if it's already a symlink pointing to the same target
			if linkInfo, err := os.Readlink(destAbs); err == nil {
				// It's a symlink, check if it points to the same target
				linkAbs, err := filepath.Abs(filepath.Join(filepath.Dir(destAbs), linkInfo))
				if err == nil {
					srcAbsResolved, err := filepath.Abs(srcAbs)
					if err == nil && linkAbs == srcAbsResolved {
						// Already linked to the same target, skip
						continue
					}
				}
				// Remove existing symlink to recreate it
				if err := os.Remove(destAbs); err != nil {
					return fmt.Errorf("failed to remove existing symlink: %w", err)
				}
			} else {
				// Destination exists but is not a symlink
				return fmt.Errorf("destination path already exists and is not a symlink: %s", destAbs)
			}
		}

		// Resolve source to absolute path for symlink
		srcAbsResolved, err := filepath.Abs(srcAbs)
		if err != nil {
			return fmt.Errorf("failed to resolve absolute path for source: %w", err)
		}

		// Create symlink (use absolute path for source)
		if err := os.Symlink(srcAbsResolved, destAbs); err != nil {
			return fmt.Errorf("failed to create symlink from %s to %s: %w", srcAbsResolved, destAbs, err)
		}
	}

	return nil
}

// shouldWaitForCommand determines if a command should be executed synchronously
// Commands that should wait: make launch, make stop, docker-compose up, scripts, etc.
// Commands that should run in background: npm run dev, go run main.go, etc.
func shouldWaitForCommand(command string) bool {
	// Commands that should complete before continuing
	waitCommands := []string{
		"make launch",
		"make stop",
		"docker-compose up",
		"docker-compose down",
		"docker compose up",
		"docker compose down",
	}

	commandLower := strings.ToLower(command)
	for _, waitCmd := range waitCommands {
		if strings.Contains(commandLower, waitCmd) {
			return true
		}
	}

	// Scripts (installer.sh, setup.sh, etc.) should execute synchronously to catch errors
	// These are typically deployment/setup scripts that should complete before continuing
	if strings.HasSuffix(commandLower, ".sh") || strings.HasPrefix(commandLower, "./") || strings.HasPrefix(commandLower, "sh ") {
		// Exclude long-running scripts that should run in background
		// If it's a simple script execution (not npm run, go run, etc.), wait for it
		if !strings.Contains(commandLower, "npm run") &&
		   !strings.Contains(commandLower, "go run") &&
		   !strings.Contains(commandLower, "python") &&
		   !strings.Contains(commandLower, "node") {
			return true
		}
	}

	// Default: run in background for long-running services
	return false
}

// parseEnvFile parses an env file content into key=value pairs
func parseEnvFile(content string) []string {
	var vars []string
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		// Trim whitespace
		line = strings.TrimSpace(line)

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Add to vars (assumes format: KEY=VALUE)
		vars = append(vars, line)
	}

	return vars
}
