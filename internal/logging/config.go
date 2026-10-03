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
	case "off", "none", "silent":
		return LogLevelOff
	default:
		return LogLevelInfo
	}
}

// InitFromEnv initializes the logger from environment variables
func InitFromEnv() {
	levelStr := os.Getenv("RAIOZ_LOG_LEVEL")
	if levelStr == "" {
		// A person at a terminal gets the failure once, formatted by the
		// CLI; the structured copy on stderr is for --log-level and for CI,
		// where logs are collected rather than read live.
		levelStr = string(LogLevelOff)
		if IsCI() {
			levelStr = string(LogLevelError)
		}
	}

	jsonFormat := IsCI() || os.Getenv("RAIOZ_LOG_JSON") == "true"

	Init(ParseLogLevel(levelStr), jsonFormat)
}
