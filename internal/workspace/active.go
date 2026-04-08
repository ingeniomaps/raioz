package workspace

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
)

const activeWorkspaceFileName = "active-workspace"

// getBaseDirForActiveWorkspace returns the base directory for storing active workspace
// Uses same logic as GetBaseDir but specifically for config files
func getBaseDirForActiveWorkspace() (string, error) {
	// Check for override environment variable
	if home := os.Getenv("RAIOZ_HOME"); home != "" {
		if err := os.MkdirAll(home, 0755); err != nil {
			return "", fmt.Errorf("failed to create RAIOZ_HOME directory '%s': %w", home, err)
		}
		return home, nil
	}

	// Try /opt/raioz-proyecto first (preferred location)
	optBase := "/opt/raioz-proyecto"
	if err := os.MkdirAll(optBase, 0755); err == nil {
		return optBase, nil
	}

	// Failed to create in /opt, use fallback
	usr, err := user.Current()
	if err != nil {
		return "", fmt.Errorf("failed to get current user: %w", err)
	}

	homeDir := usr.HomeDir
	if homeDir == "" {
		return "", fmt.Errorf("home directory is empty")
	}

	fallbackBase := filepath.Join(homeDir, ".raioz")
	if runtime.GOOS == "windows" {
		fallbackBase = filepath.Join(homeDir, ".raioz")
	}

	if err := os.MkdirAll(fallbackBase, 0755); err != nil {
		return "", fmt.Errorf("failed to create fallback directory '%s': %w", fallbackBase, err)
	}

	return fallbackBase, nil
}

// GetActiveWorkspacePath returns the path to the active workspace file
func GetActiveWorkspacePath() (string, error) {
	baseDir, err := getBaseDirForActiveWorkspace()
	if err != nil {
		return "", fmt.Errorf("failed to get base directory for active workspace: %w", err)
	}
	return filepath.Join(baseDir, activeWorkspaceFileName), nil
}

// GetActiveWorkspace returns the currently active workspace name
// Returns empty string if no workspace is active
func GetActiveWorkspace() (string, error) {
	path, err := GetActiveWorkspacePath()
	if err != nil {
		return "", err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil // No active workspace
		}
		return "", fmt.Errorf("failed to read active workspace: %w", err)
	}

	workspaceName := strings.TrimSpace(string(data))
	return workspaceName, nil
}

// SetActiveWorkspace sets the currently active workspace
func SetActiveWorkspace(workspaceName string) error {
	path, err := GetActiveWorkspacePath()
	if err != nil {
		return err
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory for active workspace: %w", err)
	}

	// Write workspace name (trimmed)
	workspaceName = strings.TrimSpace(workspaceName)
	if err := os.WriteFile(path, []byte(workspaceName+"\n"), 0644); err != nil {
		return fmt.Errorf("failed to write active workspace: %w", err)
	}

	return nil
}

// ClearActiveWorkspace removes the active workspace setting
func ClearActiveWorkspace() error {
	path, err := GetActiveWorkspacePath()
	if err != nil {
		return err
	}

	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return nil // Already cleared
		}
		return fmt.Errorf("failed to clear active workspace: %w", err)
	}

	return nil
}

// ListWorkspaces returns a list of all available workspaces
func ListWorkspaces() ([]string, error) {
	baseDir, err := GetBaseDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get base directory: %w", err)
	}

	workspacesDir := filepath.Join(baseDir, "workspaces")
	entries, err := os.ReadDir(workspacesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil // No workspaces directory yet
		}
		return nil, fmt.Errorf("failed to read workspaces directory: %w", err)
	}

	var workspaces []string
	for _, entry := range entries {
		if entry.IsDir() {
			workspaces = append(workspaces, entry.Name())
		}
	}

	return workspaces, nil
}

// DeleteWorkspace removes a workspace directory and its contents
func DeleteWorkspace(workspaceName string) error {
	baseDir, err := GetBaseDir()
	if err != nil {
		return fmt.Errorf("failed to get base directory: %w", err)
	}

	workspacePath := filepath.Join(baseDir, "workspaces", workspaceName)
	if err := validatePathInBase(workspacePath, filepath.Join(baseDir, "workspaces")); err != nil {
		return fmt.Errorf("invalid workspace path: %w", err)
	}

	if err := os.RemoveAll(workspacePath); err != nil {
		return fmt.Errorf("failed to delete workspace directory: %w", err)
	}

	return nil
}

// validatePathInBase ensures path is inside base to prevent path traversal
func validatePathInBase(path, base string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	absBase, err := filepath.Abs(base)
	if err != nil {
		return err
	}
	if !strings.HasPrefix(absPath, absBase+string(filepath.Separator)) && absPath != absBase {
		return fmt.Errorf("path %s is outside base %s", absPath, absBase)
	}
	return nil
}

// WorkspaceExists checks if a workspace exists
func WorkspaceExists(workspaceName string) (bool, error) {
	baseDir, err := GetBaseDir()
	if err != nil {
		return false, fmt.Errorf("failed to get base directory: %w", err)
	}

	workspacePath := filepath.Join(baseDir, "workspaces", workspaceName)
	info, err := os.Stat(workspacePath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to check workspace existence: %w", err)
	}

	return info.IsDir(), nil
}
