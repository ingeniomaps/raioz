package link

import (
	"fmt"
	"os"
	"path/filepath"

	"raioz/internal/workspace"
)

// IsLinked checks if a service is linked (has a symlink)
func IsLinked(servicePath string) (bool, string, error) {
	info, err := os.Lstat(servicePath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, "", nil
		}
		return false, "", fmt.Errorf("failed to stat service path: %w", err)
	}

	// Check if it's a symlink
	if info.Mode()&os.ModeSymlink != 0 {
		// It's a symlink, resolve the target
		target, err := os.Readlink(servicePath)
		if err != nil {
			return false, "", fmt.Errorf("failed to read symlink: %w", err)
		}
		// Resolve to absolute path
		if !filepath.IsAbs(target) {
			absTarget, err := filepath.Abs(filepath.Join(filepath.Dir(servicePath), target))
			if err != nil {
				return false, "", fmt.Errorf("failed to resolve symlink target: %w", err)
			}
			target = absTarget
		}
		return true, target, nil
	}

	return false, "", nil
}

// CreateLink creates a symlink from servicePath to externalPath
// If servicePath already exists and is not a symlink, returns an error
func CreateLink(servicePath string, externalPath string) error {
	// Validate that external path exists
	externalInfo, err := os.Stat(externalPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("external path does not exist: %s", externalPath)
		}
		return fmt.Errorf("failed to stat external path: %w", err)
	}

	if !externalInfo.IsDir() {
		return fmt.Errorf("external path is not a directory: %s", externalPath)
	}

	// Check if service path already exists
	if _, err := os.Lstat(servicePath); err == nil {
		// Path exists, check if it's already a symlink
		isLinked, existingTarget, err := IsLinked(servicePath)
		if err != nil {
			return fmt.Errorf("failed to check existing link: %w", err)
		}

		if isLinked {
			// Already a symlink, check if it points to the same target
			absExternal, err := filepath.Abs(externalPath)
			if err != nil {
				return fmt.Errorf("failed to resolve external path: %w", err)
			}
			absExisting, err := filepath.Abs(existingTarget)
			if err != nil {
				return fmt.Errorf("failed to resolve existing target: %w", err)
			}
			if absExternal == absExisting {
				// Already linked to the same target, no-op
				return nil
			}
			return fmt.Errorf("service path already exists as symlink pointing to: %s", existingTarget)
		}

		// Path exists but is not a symlink (it's a real directory)
		return fmt.Errorf("service path already exists as a directory (not a symlink): %s", servicePath)
	}

	// Ensure parent directory exists
	parentDir := filepath.Dir(servicePath)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		return fmt.Errorf("failed to create parent directory: %w", err)
	}

	// Resolve external path to absolute
	absExternal, err := filepath.Abs(externalPath)
	if err != nil {
		return fmt.Errorf("failed to resolve external path: %w", err)
	}

	// Create symlink
	if err := os.Symlink(absExternal, servicePath); err != nil {
		return fmt.Errorf("failed to create symlink: %w", err)
	}

	return nil
}

// RemoveLink removes a symlink from servicePath
// Returns error if path doesn't exist or is not a symlink
func RemoveLink(servicePath string) error {
	// Check if path exists
	info, err := os.Lstat(servicePath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("service path does not exist: %s", servicePath)
		}
		return fmt.Errorf("failed to stat service path: %w", err)
	}

	// Check if it's a symlink
	if info.Mode()&os.ModeSymlink == 0 {
		return fmt.Errorf("service path is not a symlink: %s", servicePath)
	}

	// Remove symlink
	if err := os.Remove(servicePath); err != nil {
		return fmt.Errorf("failed to remove symlink: %w", err)
	}

	return nil
}

// GetServiceLinkPath returns the path where a service symlink would be created
func GetServiceLinkPath(ws *workspace.Workspace, serviceName string, svc interface{}) (string, error) {
	// We need to use workspace.GetServicePath, but we can't import config here
	// So we'll use a function that accepts the service path directly
	// This will be called from cmd/link.go which has access to config
	return "", fmt.Errorf("use workspace.GetServicePath instead")
}
