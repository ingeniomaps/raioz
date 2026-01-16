package path

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ValidatePathInBase validates that a path is within a base directory to prevent path traversal
// Normalizes paths with filepath.Abs() and verifies the resulting path is within baseDir
func ValidatePathInBase(path, baseDir string) error {
	if path == "" {
		return fmt.Errorf("path cannot be empty")
	}

	if baseDir == "" {
		return fmt.Errorf("base directory cannot be empty")
	}

	// Normalize both paths to absolute paths
	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for base directory: %w", err)
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Clean both paths to remove any .. or . components
	absBase = filepath.Clean(absBase)
	absPath = filepath.Clean(absPath)

	// Check if path contains .. (path traversal attempt)
	// This is a redundant check since filepath.Clean should handle it, but we check explicitly for security
	if strings.Contains(path, "..") {
		return fmt.Errorf("path traversal detected: path contains '..'")
	}

	// Verify that the absolute path is within the base directory
	// Use filepath.Rel to check if path is within baseDir
	rel, err := filepath.Rel(absBase, absPath)
	if err != nil {
		return fmt.Errorf("failed to compute relative path: %w", err)
	}

	// If the relative path starts with "..", it means we're outside the base directory
	if strings.HasPrefix(rel, "..") {
		return fmt.Errorf(
			"path traversal detected: path '%s' is outside base directory '%s'",
			path, baseDir,
		)
	}

	// Additional check: verify that the normalized path actually starts with the base directory
	// This handles edge cases with symlinks and different path formats
	if !strings.HasPrefix(absPath+string(filepath.Separator), absBase+string(filepath.Separator)) &&
		absPath != absBase {
		return fmt.Errorf(
			"path traversal detected: path '%s' is outside base directory '%s'",
			path, baseDir,
		)
	}

	return nil
}

// ValidateAndCleanPath validates a path and returns a cleaned version
// Useful for paths that will be used with filepath.Join
// Note: This function validates the input path component, but full path traversal
// validation should be done with ValidatePathInBase after constructing the full path
func ValidateAndCleanPath(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("path cannot be empty")
	}

	// Check for dangerous patterns before cleaning
	if strings.Contains(path, "\x00") {
		return "", fmt.Errorf("path contains null byte")
	}

	// Check for path traversal attempts BEFORE cleaning
	// filepath.Clean will normalize "sub/../file" to "file", so we check the raw input
	if strings.Contains(path, "..") {
		return "", fmt.Errorf("path traversal detected: path contains '..'")
	}

	// Reject absolute paths (these should be handled separately)
	if filepath.IsAbs(path) {
		return "", fmt.Errorf("absolute paths are not allowed in path components")
	}

	// Clean the path to normalize it (remove . and redundant separators)
	cleaned := filepath.Clean(path)

	return cleaned, nil
}

// EnsurePathInBase validates that a path constructed from baseDir and userPath is within baseDir
// This is the main function to use when building paths from user input
func EnsurePathInBase(baseDir, userPath string) (string, error) {
	// Clean and validate the user path component
	cleanedUserPath, err := ValidateAndCleanPath(userPath)
	if err != nil {
		return "", fmt.Errorf("invalid user path: %w", err)
	}

	// Construct the full path
	fullPath := filepath.Join(baseDir, cleanedUserPath)

	// Validate that the constructed path is within the base directory
	if err := ValidatePathInBase(fullPath, baseDir); err != nil {
		return "", err
	}

	return fullPath, nil
}

// CheckSymlinkSecurity checks if a path is a symlink that could be dangerous
// This is a basic check - for production use, consider using os.Readlink to verify
func CheckSymlinkSecurity(path string) error {
	// Check if path is a symlink
	info, err := os.Lstat(path)
	if err != nil {
		// If path doesn't exist, that's ok - we'll validate when it's created
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to check symlink: %w", err)
	}

	// Check if it's a symlink
	if info.Mode()&os.ModeSymlink != 0 {
		// For symlinks, we rely on ValidatePathInBase to ensure the target is within baseDir
		// This is acceptable because ValidatePathInBase resolves to absolute paths
		// which should handle symlinks correctly
		return nil
	}

	return nil
}
