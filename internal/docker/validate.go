package docker

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ValidateComposePath validates a docker compose file path to prevent command injection.
// Supports multi-file specs (colon-separated) by validating each segment individually.
func ValidateComposePath(path string) error {
	if path == "" {
		return fmt.Errorf("compose path cannot be empty")
	}

	parts := SplitComposePaths(path)
	if len(parts) == 0 {
		return fmt.Errorf("compose path cannot be empty")
	}

	for _, part := range parts {
		if err := validateSingleComposePath(part); err != nil {
			return err
		}
	}
	return nil
}

// validateSingleComposePath runs injection-safety checks on one file path.
func validateSingleComposePath(path string) error {
	// Clean the path to normalize it
	cleaned := filepath.Clean(path)

	// Check for dangerous characters that could be used for command injection
	// (colon is intentionally excluded — it is the raioz multi-file separator)
	dangerousChars := []string{";", "|", "&", "$", "`", "\n", "\r", "\t"}
	for _, char := range dangerousChars {
		if strings.Contains(path, char) {
			return fmt.Errorf("compose path contains dangerous character: %q", char)
		}
	}

	// Maximum path length (conservative limit)
	const maxPathLength = 4096
	if len(cleaned) > maxPathLength {
		return fmt.Errorf("compose path exceeds maximum length of %d characters", maxPathLength)
	}

	// Check for null bytes (path injection)
	if strings.Contains(path, "\x00") {
		return fmt.Errorf("compose path contains null byte")
	}

	return nil
}
