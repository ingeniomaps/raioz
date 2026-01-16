package env

import (
	"strings"
)

// sensitiveKeyPatterns contains patterns that indicate a sensitive environment variable key
// Keys matching these patterns (case-insensitive, substring match) will have their values redacted
var sensitiveKeyPatterns = []string{
	"PASSWORD",
	"SECRET",
	"TOKEN",
	"API_KEY",
	"AWS_SECRET",
	"PRIVATE_KEY",
	"ACCESS_KEY",
	"CREDENTIAL",
	"PASSWD",
	"PWD",
	"AUTH",
	"SESSION",
	"COOKIE",
}

// isSensitiveKey checks if an environment variable key matches any sensitive pattern
func isSensitiveKey(key string) bool {
	upperKey := strings.ToUpper(key)
	for _, pattern := range sensitiveKeyPatterns {
		if strings.Contains(upperKey, pattern) {
			return true
		}
	}
	return false
}

// SanitizeEnvValue sanitizes an environment variable value if the key is sensitive
// Returns "***REDACTED***" for sensitive keys, otherwise returns the original value
func SanitizeEnvValue(key, value string) string {
	if isSensitiveKey(key) {
		return "***REDACTED***"
	}
	return value
}

// SanitizeEnvMap sanitizes all values in an environment variable map
// Returns a new map with sensitive values redacted
func SanitizeEnvMap(env map[string]string) map[string]string {
	sanitized := make(map[string]string)
	for key, value := range env {
		sanitized[key] = SanitizeEnvValue(key, value)
	}
	return sanitized
}

// SanitizeEnvString sanitizes a string that might contain environment variable values
// This is useful for sanitizing error messages or log entries
// It checks for common patterns like "KEY=value" and sanitizes them
func SanitizeEnvString(s string) string {
	// Split by newlines to handle multi-line strings
	lines := strings.Split(s, "\n")
	var sanitizedLines []string

	for _, line := range lines {
		// Check if line looks like KEY=VALUE
		if idx := strings.Index(line, "="); idx > 0 {
			key := strings.TrimSpace(line[:idx])
			if isSensitiveKey(key) {
				sanitizedLines = append(sanitizedLines, key+"=***REDACTED***")
				continue
			}
		}
		sanitizedLines = append(sanitizedLines, line)
	}

	return strings.Join(sanitizedLines, "\n")
}
