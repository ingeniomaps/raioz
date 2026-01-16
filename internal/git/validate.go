package git

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// validateGitInput validates git branch and repo inputs to prevent command injection
func validateGitInput(branch, repo string) error {
	// Validate branch
	if err := validateBranch(branch); err != nil {
		return fmt.Errorf("invalid branch '%s': %w", branch, err)
	}

	// Validate repo
	if err := validateRepo(repo); err != nil {
		return fmt.Errorf("invalid repository URL '%s': %w", repo, err)
	}

	return nil
}

// validateBranch validates a git branch name
// Allows: alphanumeric characters, hyphens, slashes, underscores
// Rejects: dangerous characters like ; | & $ ` \n
func validateBranch(branch string) error {
	if branch == "" {
		return fmt.Errorf("branch cannot be empty")
	}

	// Maximum length for branch names (git limit is 255, but we'll be conservative)
	const maxBranchLength = 255
	if len(branch) > maxBranchLength {
		return fmt.Errorf("branch name exceeds maximum length of %d characters", maxBranchLength)
	}

	// Check for dangerous characters that could be used for command injection
	dangerousChars := []string{";", "|", "&", "$", "`", "\n", "\r", "\t"}
	for _, char := range dangerousChars {
		if strings.Contains(branch, char) {
			return fmt.Errorf("branch contains dangerous character: %q", char)
		}
	}

	// Validate branch name format: alphanumeric, hyphens, slashes, underscores, dots
	// Git allows refs/heads/ prefix, feature/branch format, etc.
	branchPattern := `^[a-zA-Z0-9/\-_.]+$`
	matched, err := regexp.MatchString(branchPattern, branch)
	if err != nil {
		return fmt.Errorf("failed to validate branch pattern: %w", err)
	}

	if !matched {
		return fmt.Errorf(
			"branch name contains invalid characters. "+
				"Only alphanumeric characters, hyphens, slashes, underscores, and dots are allowed",
		)
	}

	return nil
}

// validateRepo validates a git repository URL
// Supports: ssh://, https://, git@ (SSH format), file://
// Rejects: dangerous characters that could be used for command injection
func validateRepo(repo string) error {
	if repo == "" {
		return fmt.Errorf("repository URL cannot be empty")
	}

	// Maximum length for URLs (conservative limit)
	const maxURLLength = 2048
	if len(repo) > maxURLLength {
		return fmt.Errorf("repository URL exceeds maximum length of %d characters", maxURLLength)
	}

	// Check for dangerous characters that could be used for command injection
	dangerousChars := []string{";", "|", "&", "$", "`", "\n", "\r", "\t"}
	for _, char := range dangerousChars {
		if strings.Contains(repo, char) {
			return fmt.Errorf("repository URL contains dangerous character: %q", char)
		}
	}

	// Normalize repo URL (trim whitespace)
	repo = strings.TrimSpace(repo)

	// Validate URL format - must start with valid protocol or SSH format
	validPrefixes := []string{
		"ssh://",
		"https://",
		"http://",
		"git@",
		"file://",
	}

	hasValidPrefix := false
	for _, prefix := range validPrefixes {
		if strings.HasPrefix(repo, prefix) {
			hasValidPrefix = true
			break
		}
	}

	if !hasValidPrefix {
		// Reject invalid protocols
		return fmt.Errorf(
			"repository URL format is invalid. "+
				"Must start with ssh://, https://, http://, git@, or file://",
		)
	}

	// Additional validation: for git@ format, check structure
	if strings.HasPrefix(repo, "git@") {
		// git@host:path format
		if !strings.Contains(repo, ":") {
			return fmt.Errorf("git@ URL format must be git@host:path")
		}
		// Check for double colon (which could be problematic)
		if strings.Count(repo, ":") > 1 {
			return fmt.Errorf("git@ URL format is invalid: multiple colons")
		}
	}

	return nil
}

// validatePath validates a file path to prevent command injection
// Uses filepath.Clean to normalize the path
func validatePath(path string) error {
	if path == "" {
		return fmt.Errorf("path cannot be empty")
	}

	// Clean the path to normalize it
	cleaned := filepath.Clean(path)

	// Check for dangerous characters that could be used for command injection
	dangerousChars := []string{";", "|", "&", "$", "`", "\n", "\r", "\t"}
	for _, char := range dangerousChars {
		if strings.Contains(path, char) {
			return fmt.Errorf("path contains dangerous character: %q", char)
		}
	}

	// Maximum path length (conservative limit)
	const maxPathLength = 4096
	if len(cleaned) > maxPathLength {
		return fmt.Errorf("path exceeds maximum length of %d characters", maxPathLength)
	}

	// Check for null bytes (path injection)
	if strings.Contains(path, "\x00") {
		return fmt.Errorf("path contains null byte")
	}

	return nil
}
