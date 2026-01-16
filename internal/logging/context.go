package logging

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"time"
)

// ContextKey is a type for context keys
type ContextKey string

const (
	// RequestIDKey is the key for request ID in context
	RequestIDKey ContextKey = "request_id"
	// OperationKey is the key for operation name in context
	OperationKey ContextKey = "operation"
	// ProjectKey is the key for project name in context
	ProjectKey ContextKey = "project"
	// ServiceKey is the key for service name in context
	ServiceKey ContextKey = "service"
)

// generateRequestID generates a unique request ID
func generateRequestID() string {
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to timestamp-based ID if crypto/rand fails
		return time.Now().Format("20060102150405") + "-" + hex.EncodeToString([]byte(time.Now().String()))[:8]
	}
	return hex.EncodeToString(bytes)
}

// WithRequestID adds a request ID to the context
func WithRequestID(ctx context.Context) context.Context {
	requestID := generateRequestID()
	return context.WithValue(ctx, RequestIDKey, requestID)
}

// GetRequestID retrieves the request ID from context
func GetRequestID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if id, ok := ctx.Value(RequestIDKey).(string); ok {
		return id
	}
	return ""
}

// WithOperation adds an operation name to the context
func WithOperation(ctx context.Context, operation string) context.Context {
	return context.WithValue(ctx, OperationKey, operation)
}

// GetOperation retrieves the operation name from context
func GetOperation(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if op, ok := ctx.Value(OperationKey).(string); ok {
		return op
	}
	return ""
}

// WithProject adds a project name to the context
func WithProject(ctx context.Context, project string) context.Context {
	return context.WithValue(ctx, ProjectKey, project)
}

// GetProject retrieves the project name from context
func GetProject(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if project, ok := ctx.Value(ProjectKey).(string); ok {
		return project
	}
	return ""
}

// WithService adds a service name to the context
func WithService(ctx context.Context, service string) context.Context {
	return context.WithValue(ctx, ServiceKey, service)
}

// GetService retrieves the service name from context
func GetService(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if service, ok := ctx.Value(ServiceKey).(string); ok {
		return service
	}
	return ""
}

// WithLogContext creates a logger with context values as attributes
func WithLogContext(ctx context.Context) *slog.Logger {
	if Logger == nil {
		Default()
	}

	args := []any{}

	if requestID := GetRequestID(ctx); requestID != "" {
		args = append(args, "request_id", requestID)
	}

	if operation := GetOperation(ctx); operation != "" {
		args = append(args, "operation", operation)
	}

	if project := GetProject(ctx); project != "" {
		args = append(args, "project", project)
	}

	if service := GetService(ctx); service != "" {
		args = append(args, "service", service)
	}

	if len(args) > 0 {
		return Logger.With(args...)
	}

	return Logger
}

// DebugWithContext logs a debug message with context
func DebugWithContext(ctx context.Context, msg string, args ...any) {
	logger := WithLogContext(ctx)
	logger.Debug(msg, args...)
}

// InfoWithContext logs an info message with context
func InfoWithContext(ctx context.Context, msg string, args ...any) {
	logger := WithLogContext(ctx)
	logger.Info(msg, args...)
}

// WarnWithContext logs a warning message with context
func WarnWithContext(ctx context.Context, msg string, args ...any) {
	logger := WithLogContext(ctx)
	logger.Warn(msg, args...)
}

// ErrorWithContext logs an error message with context
func ErrorWithContext(ctx context.Context, msg string, args ...any) {
	logger := WithLogContext(ctx)
	logger.Error(msg, args...)
}

// LogOperationStart logs the start of an operation
func LogOperationStart(ctx context.Context, operation string, args ...any) {
	logger := WithLogContext(ctx)
	allArgs := append([]any{"operation", operation, "status", "start"}, args...)
	logger.Info("Operation started", allArgs...)
}

// LogOperationEnd logs the end of an operation with duration
func LogOperationEnd(ctx context.Context, operation string, startTime time.Time, err error, args ...any) {
	logger := WithLogContext(ctx)
	duration := time.Since(startTime)

	allArgs := append([]any{
		"operation", operation,
		"status", "end",
		"duration_ms", duration.Milliseconds(),
	}, args...)

	if err != nil {
		allArgs = append(allArgs, "error", err.Error())
		logger.Error("Operation failed", allArgs...)
	} else {
		// Use Debug level for successful operations - not useful for end users
		logger.Debug("Operation completed", allArgs...)
	}
}

// LogCriticalOperation logs a critical operation with full context
func LogCriticalOperation(ctx context.Context, level LogLevel, msg string, args ...any) {
	logger := WithLogContext(ctx)

	// Add timestamp
	allArgs := append([]any{"timestamp", time.Now().Format(time.RFC3339)}, args...)

	switch level {
	case LogLevelDebug:
		logger.Debug(msg, allArgs...)
	case LogLevelInfo:
		logger.Info(msg, allArgs...)
	case LogLevelWarn:
		logger.Warn(msg, allArgs...)
	case LogLevelError:
		logger.Error(msg, allArgs...)
	default:
		logger.Info(msg, allArgs...)
	}
}
