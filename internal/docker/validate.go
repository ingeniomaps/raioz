package docker

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ValidateComposePath validates a docker compose file path to prevent command injection
func ValidateComposePath(path string) error {
	if path == "" {
		return fmt.Errorf("compose path cannot be empty")
	}

	// Clean the path to normalize it
	cleaned := filepath.Clean(path)

	// Check for dangerous characters that could be used for command injection
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
