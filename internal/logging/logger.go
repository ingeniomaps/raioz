package logging

import (
	"log/slog"
	"os"
	"strings"
)

// LogLevel represents the logging level
type LogLevel string

const (
	LogLevelDebug LogLevel = "debug"
	LogLevelInfo  LogLevel = "info"
	LogLevelWarn  LogLevel = "warn"
	LogLevelError LogLevel = "error"
)

var (
	// Logger is the global logger instance
	Logger *slog.Logger
	// logLevel stores the current log level
	logLevel = LogLevelInfo
	// jsonFormat stores whether JSON format is enabled
	jsonFormat = false
)

// Init initializes the logger with the given level and format
func Init(level LogLevel, json bool) {
	logLevel = level
	jsonFormat = json

	var handler slog.Handler
	opts := &slog.HandlerOptions{
		Level: parseLevel(level),
	}

	// Send structured logs to stderr to avoid cluttering user output
	// User-friendly messages should use output.Print* functions instead
	if json {
		handler = slog.NewJSONHandler(os.Stderr, opts)
	} else {
		handler = slog.NewTextHandler(os.Stderr, opts)
	}

	Logger = slog.New(handler)
}

// SetLevel updates the log level
func SetLevel(level LogLevel) {
	logLevel = level
	if Logger != nil {
		Init(level, jsonFormat)
	}
}

// SetJSONFormat sets whether to use JSON format
func SetJSONFormat(json bool) {
	jsonFormat = json
	if Logger != nil {
		Init(logLevel, json)
	}
}

// parseLevel converts LogLevel to slog.Level
func parseLevel(level LogLevel) slog.Level {
	switch strings.ToLower(string(level)) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// IsDebugEnabled returns true if debug level is enabled
func IsDebugEnabled() bool {
	return logLevel == LogLevelDebug
}

// IsJSONFormat returns true if JSON format is enabled
func IsJSONFormat() bool {
	return jsonFormat
}

// Debug logs a debug message
func Debug(msg string, args ...any) {
	if Logger != nil {
		Logger.Debug(msg, args...)
	}
}

// Info logs an info message
func Info(msg string, args ...any) {
	if Logger != nil {
		Logger.Info(msg, args...)
	}
}

// Warn logs a warning message
func Warn(msg string, args ...any) {
	if Logger != nil {
		Logger.Warn(msg, args...)
	}
}

// Error logs an error message
func Error(msg string, args ...any) {
	if Logger != nil {
		Logger.Error(msg, args...)
	}
}

// Default initializes the logger with default settings if not already initialized
func Default() {
	if Logger == nil {
		Init(LogLevelInfo, false)
	}
}
