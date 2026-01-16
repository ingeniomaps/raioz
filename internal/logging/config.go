package logging

import (
	"os"
	"strings"
)

// IsCI returns true if running in a CI environment
func IsCI() bool {
	ciVars := []string{
		"CI",
		"CONTINUOUS_INTEGRATION",
		"GITHUB_ACTIONS",
		"GITLAB_CI",
		"JENKINS_URL",
		"TRAVIS",
		"CIRCLECI",
	}

	for _, env := range ciVars {
		if os.Getenv(env) != "" {
			return true
		}
	}
	return false
}

// ParseLogLevel parses a log level string and returns LogLevel
func ParseLogLevel(level string) LogLevel {
	level = strings.ToLower(strings.TrimSpace(level))
	switch level {
	case "debug":
		return LogLevelDebug
	case "info":
		return LogLevelInfo
	case "warn", "warning":
		return LogLevelWarn
	case "error":
		return LogLevelError
	default:
		return LogLevelInfo
	}
}

// InitFromEnv initializes the logger from environment variables
func InitFromEnv() {
	levelStr := os.Getenv("RAIOZ_LOG_LEVEL")
	if levelStr == "" {
		// Default to error level to avoid cluttering user output
		// Structured logs are sent to stderr, user-friendly messages use output.Print*
		// Users can use --log-level debug or --log-level info if they need detailed logs
		levelStr = "error"
	}

	jsonFormat := IsCI() || os.Getenv("RAIOZ_LOG_JSON") == "true"

	Init(ParseLogLevel(levelStr), jsonFormat)
}
