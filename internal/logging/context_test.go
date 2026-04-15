package logging

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestGenerateRequestID(t *testing.T) {
	id1 := generateRequestID()
	id2 := generateRequestID()

	if id1 == "" {
		t.Error("expected non-empty request ID")
	}
	if id1 == id2 {
		t.Errorf("expected unique IDs, got %q twice", id1)
	}
}

func TestWithRequestID_AndGet(t *testing.T) {
	ctx := context.Background()
	ctx2 := WithRequestID(ctx)

	id := GetRequestID(ctx2)
	if id == "" {
		t.Error("expected request ID to be set")
	}

	// Base context has no ID
	if got := GetRequestID(ctx); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestGetRequestID_NilContext(t *testing.T) {
	if got := GetRequestID(nil); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestWithOperation_AndGet(t *testing.T) {
	ctx := WithOperation(context.Background(), "up")
	if got := GetOperation(ctx); got != "up" {
		t.Errorf("expected 'up', got %q", got)
	}

	if got := GetOperation(context.Background()); got != "" {
		t.Errorf("expected empty, got %q", got)
	}

	if got := GetOperation(nil); got != "" {
		t.Errorf("expected empty for nil ctx, got %q", got)
	}
}

func TestWithProject_AndGet(t *testing.T) {
	ctx := WithProject(context.Background(), "ecommerce")
	if got := GetProject(ctx); got != "ecommerce" {
		t.Errorf("expected 'ecommerce', got %q", got)
	}

	if got := GetProject(context.Background()); got != "" {
		t.Errorf("expected empty, got %q", got)
	}

	if got := GetProject(nil); got != "" {
		t.Errorf("expected empty for nil ctx, got %q", got)
	}
}

func TestWithService_AndGet(t *testing.T) {
	ctx := WithService(context.Background(), "api")
	if got := GetService(ctx); got != "api" {
		t.Errorf("expected 'api', got %q", got)
	}

	if got := GetService(context.Background()); got != "" {
		t.Errorf("expected empty, got %q", got)
	}

	if got := GetService(nil); got != "" {
		t.Errorf("expected empty for nil ctx, got %q", got)
	}
}

func TestWithLogContext_EmptyContext(t *testing.T) {
	Init(LogLevelDebug, false)
	logger := WithLogContext(context.Background())
	if logger == nil {
		t.Error("expected non-nil logger")
	}
}

func TestWithLogContext_FullContext(t *testing.T) {
	Init(LogLevelDebug, false)

	ctx := context.Background()
	ctx = WithRequestID(ctx)
	ctx = WithOperation(ctx, "up")
	ctx = WithProject(ctx, "proj")
	ctx = WithService(ctx, "api")

	logger := WithLogContext(ctx)
	if logger == nil {
		t.Error("expected non-nil logger")
	}
}

func TestWithLogContext_NilLogger(t *testing.T) {
	oldLogger := Logger
	Logger = nil
	defer func() { Logger = oldLogger }()

	logger := WithLogContext(context.Background())
	if logger == nil {
		t.Error("expected Default() to initialize logger")
	}
}

func TestContextLoggingFunctions(t *testing.T) {
	Init(LogLevelDebug, false)

	ctx := WithOperation(context.Background(), "test-op")
	ctx = WithProject(ctx, "test-proj")

	// These just need to not panic
	tests := []struct {
		name string
		fn   func()
	}{
		{"DebugWithContext", func() { DebugWithContext(ctx, "msg", "k", "v") }},
		{"InfoWithContext", func() { InfoWithContext(ctx, "msg", "k", "v") }},
		{"WarnWithContext", func() { WarnWithContext(ctx, "msg", "k", "v") }},
		{"ErrorWithContext", func() { ErrorWithContext(ctx, "msg", "k", "v") }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.fn()
		})
	}
}

func TestLogOperationStart(t *testing.T) {
	Init(LogLevelDebug, false)

	ctx := WithRequestID(context.Background())
	LogOperationStart(ctx, "deploy", "user", "alice")
	// Just needs to not panic
}

func TestLogOperationEnd_Success(t *testing.T) {
	Init(LogLevelDebug, false)

	ctx := context.Background()
	start := time.Now().Add(-10 * time.Millisecond)
	LogOperationEnd(ctx, "deploy", start, nil, "k", "v")
}

func TestLogOperationEnd_WithError(t *testing.T) {
	Init(LogLevelDebug, false)

	ctx := context.Background()
	start := time.Now().Add(-5 * time.Millisecond)
	LogOperationEnd(ctx, "deploy", start, errors.New("boom"), "k", "v")
}

func TestLogCriticalOperation_AllLevels(t *testing.T) {
	Init(LogLevelDebug, false)

	ctx := WithProject(context.Background(), "p")

	levels := []LogLevel{
		LogLevelDebug,
		LogLevelInfo,
		LogLevelWarn,
		LogLevelError,
		LogLevel("unknown"),
	}

	for _, lvl := range levels {
		t.Run(string(lvl), func(t *testing.T) {
			LogCriticalOperation(ctx, lvl, "critical msg", "k", "v")
		})
	}
}

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		input string
		want  LogLevel
	}{
		{"debug", LogLevelDebug},
		{"DEBUG", LogLevelDebug},
		{"info", LogLevelInfo},
		{"warn", LogLevelWarn},
		{"warning", LogLevelWarn},
		{"error", LogLevelError},
		{"  info  ", LogLevelInfo},
		{"nonsense", LogLevelInfo},
		{"", LogLevelInfo},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := ParseLogLevel(tt.input); got != tt.want {
				t.Errorf("ParseLogLevel(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsCI(t *testing.T) {
	// Clear all known CI vars (t.Setenv restores automatically)
	vars := []string{
		"CI", "CONTINUOUS_INTEGRATION", "GITHUB_ACTIONS", "GITLAB_CI",
		"JENKINS_URL", "TRAVIS", "CIRCLECI",
	}
	for _, v := range vars {
		t.Setenv(v, "")
	}

	if IsCI() {
		t.Error("expected IsCI() = false with all vars empty")
	}

	t.Setenv("CI", "true")
	if !IsCI() {
		t.Error("expected IsCI() = true with CI=true")
	}
}
