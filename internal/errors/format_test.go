package errors

import (
	stderrors "errors"
	"fmt"
	"strings"
	"testing"
)

func TestFormatError_RaiozError(t *testing.T) {
	err := New(ErrCodeInvalidConfig, "bad config").
		WithContext("file", "raioz.yaml").
		WithSuggestion("fix it")

	got := FormatError(err)
	if !strings.Contains(got, "bad config") {
		t.Errorf("expected message in output: %s", got)
	}
	if !strings.Contains(got, "raioz.yaml") {
		t.Errorf("expected context in output: %s", got)
	}
	if !strings.Contains(got, "fix it") {
		t.Errorf("expected suggestion in output: %s", got)
	}
}

func TestFormatError_WrappedRaiozError(t *testing.T) {
	inner := New(ErrCodeDockerNotRunning, "docker down")
	wrapped := fmt.Errorf("outer: %w", inner)

	got := FormatError(wrapped)
	if !strings.Contains(got, "docker down") {
		t.Errorf("expected wrapped error to be formatted as RaiozError: %s", got)
	}
}

func TestFormatError_RegularError(t *testing.T) {
	got := FormatError(stderrors.New("plain error"))
	if !strings.Contains(got, "plain error") {
		t.Errorf("expected message: %s", got)
	}
}

func TestFormatMultipleErrors_Empty(t *testing.T) {
	got := FormatMultipleErrors(nil)
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestFormatMultipleErrors_Multiple(t *testing.T) {
	errs := []error{
		New(ErrCodeInvalidConfig, "error 1"),
		New(ErrCodePortConflict, "error 2"),
		stderrors.New("error 3"),
	}
	got := FormatMultipleErrors(errs)
	if !strings.Contains(got, "error 1") {
		t.Error("missing error 1")
	}
	if !strings.Contains(got, "error 2") {
		t.Error("missing error 2")
	}
	if !strings.Contains(got, "error 3") {
		t.Error("missing error 3")
	}
	if !strings.Contains(got, "3 error") {
		t.Error("missing count")
	}
}

func TestAs_Nil(t *testing.T) {
	var target *RaiozError
	if As(nil, &target) {
		t.Error("expected false for nil")
	}
}

func TestAs_Direct(t *testing.T) {
	orig := New(ErrCodeInvalidConfig, "msg")
	var target *RaiozError
	if !As(orig, &target) {
		t.Error("expected true")
	}
	if target.Code != ErrCodeInvalidConfig {
		t.Errorf("wrong code: %s", target.Code)
	}
}

func TestAs_Wrapped(t *testing.T) {
	inner := New(ErrCodePortConflict, "port busy")
	wrapped := fmt.Errorf("context: %w", inner)

	var target *RaiozError
	if !As(wrapped, &target) {
		t.Error("expected true for wrapped error")
	}
	if target.Code != ErrCodePortConflict {
		t.Errorf("wrong code: %s", target.Code)
	}
}

func TestAs_NoMatch(t *testing.T) {
	var target *RaiozError
	if As(stderrors.New("plain"), &target) {
		t.Error("expected false for non-raioz error")
	}
}

func TestRaiozError_Unwrap(t *testing.T) {
	orig := stderrors.New("original")
	err := New(ErrCodeInternalError, "wrapper").WithError(orig)

	if err.Unwrap() != orig {
		t.Error("Unwrap should return original error")
	}
}

func TestRaiozError_Unwrap_Nil(t *testing.T) {
	err := New(ErrCodeInternalError, "no inner")
	if err.Unwrap() != nil {
		t.Error("Unwrap should return nil when no original")
	}
}

func TestRaiozError_Format_Minimal(t *testing.T) {
	err := New(ErrCodeInvalidConfig, "just a message")
	got := err.Format()
	if !strings.Contains(got, "just a message") {
		t.Errorf("format missing message: %s", got)
	}
}

func TestRaiozError_Format_WithOriginal(t *testing.T) {
	orig := stderrors.New("root cause")
	err := New(ErrCodeInternalError, "wrapper").WithError(orig)
	got := err.Format()
	if !strings.Contains(got, "root cause") {
		t.Errorf("format missing original error: %s", got)
	}
}

func TestRaiozError_WithContext_NilMap(t *testing.T) {
	// Simulate a RaiozError with nil context map
	err := &RaiozError{
		Code:    ErrCodeInvalidConfig,
		Message: "test",
		Context: nil,
	}
	err.WithContext("key", "val")
	if err.Context["key"] != "val" {
		t.Error("WithContext should initialize map")
	}
}

func TestRaiozError_Chain(t *testing.T) {
	// Verify all builder methods return *RaiozError (for chaining)
	err := New(ErrCodeInvalidConfig, "test").
		WithContext("k", "v").
		WithSuggestion("try this").
		WithError(stderrors.New("inner"))

	if err.Code != ErrCodeInvalidConfig {
		t.Error("code lost")
	}
	if err.Context["k"] != "v" {
		t.Error("context lost")
	}
	if err.Suggestion != "try this" {
		t.Error("suggestion lost")
	}
	if err.OriginalErr == nil {
		t.Error("original error lost")
	}
}

// Tests for orchestrator-specific error constructors
func TestRuntimeNotDetected(t *testing.T) {
	err := RuntimeNotDetected("api", "/path")
	if err.Code != ErrCodeRuntimeNotDetected {
		t.Error("wrong code")
	}
	if err.Context["service"] != "api" {
		t.Error("service missing in context")
	}
	if err.Context["path"] != "/path" {
		t.Error("path missing in context")
	}
}

func TestRuntimeNotInstalled(t *testing.T) {
	err := RuntimeNotInstalled("go", "go")
	if err.Code != ErrCodeRuntimeNotInstalled {
		t.Error("wrong code")
	}
}

func TestServiceStartFailed_KnownRuntime(t *testing.T) {
	inner := stderrors.New("crash")
	for _, rt := range []string{"compose", "dockerfile", "npm", "go", "make", "python", "rust", "image"} {
		t.Run(rt, func(t *testing.T) {
			err := ServiceStartFailed("svc", rt, inner)
			if err.Code != ErrCodeServiceStartFailed {
				t.Error("wrong code")
			}
			if err.Suggestion == "" {
				t.Errorf("expected runtime-specific suggestion for %s", rt)
			}
		})
	}
}

func TestServiceStartFailed_UnknownRuntime(t *testing.T) {
	err := ServiceStartFailed("svc", "unknown", stderrors.New("x"))
	if err.Suggestion == "" {
		t.Error("expected fallback suggestion")
	}
}

func TestDependencyStartFailed(t *testing.T) {
	err := DependencyStartFailed("postgres", "postgres:16", stderrors.New("fail"))
	if err.Code != ErrCodeDepStartFailed {
		t.Error("wrong code")
	}
}

func TestProxyStartFailed(t *testing.T) {
	err := ProxyStartFailed(stderrors.New("caddy fail"))
	if err.Code != ErrCodeProxyStartFailed {
		t.Error("wrong code")
	}
}

func TestPathNotFound(t *testing.T) {
	err := PathNotFound("svc", "/missing")
	if err.Code != ErrCodePathNotFound {
		t.Error("wrong code")
	}
	if err.Context["path"] != "/missing" {
		t.Error("path missing")
	}
}

func TestDevSwapFailed(t *testing.T) {
	err := DevSwapFailed("postgres", "promote", stderrors.New("fail"))
	if err.Code != ErrCodeDevSwapFailed {
		t.Error("wrong code")
	}
}

func TestPreHookFailed(t *testing.T) {
	err := PreHookFailed("./fetch-secrets.sh", stderrors.New("fail"))
	if err.Code != ErrCodePreHookFailed {
		t.Error("wrong code")
	}
}

func TestYAMLParseFailed(t *testing.T) {
	err := YAMLParseFailed("raioz.yaml", stderrors.New("bad syntax"))
	if err.Code != ErrCodeYAMLParseFailed {
		t.Error("wrong code")
	}
}
