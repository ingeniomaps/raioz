package errors

import (
	"errors"
	"testing"
)

func TestRaiozError(t *testing.T) {
	t.Run("basic error", func(t *testing.T) {
		err := New(ErrCodeInvalidConfig, "test error")
		if err.Code != ErrCodeInvalidConfig {
			t.Errorf("Expected code %s, got %s", ErrCodeInvalidConfig, err.Code)
		}
		if err.Error() != "test error" {
			t.Errorf("Expected message 'test error', got %s", err.Error())
		}
	})

	t.Run("with context", func(t *testing.T) {
		err := New(ErrCodeInvalidConfig, "test error").
			WithContext("key1", "value1").
			WithContext("key2", 42)

		if len(err.Context) != 2 {
			t.Errorf("Expected 2 context items, got %d", len(err.Context))
		}
		if err.Context["key1"] != "value1" {
			t.Errorf("Expected context key1='value1', got %v", err.Context["key1"])
		}
	})

	t.Run("with suggestion", func(t *testing.T) {
		err := New(ErrCodeInvalidConfig, "test error").
			WithSuggestion("fix it")

		if err.Suggestion != "fix it" {
			t.Errorf("Expected suggestion 'fix it', got %s", err.Suggestion)
		}
	})

	t.Run("with error", func(t *testing.T) {
		originalErr := errors.New("original error")
		err := New(ErrCodeInvalidConfig, "test error").
			WithError(originalErr)

		if err.OriginalErr != originalErr {
			t.Errorf("Expected original error, got %v", err.OriginalErr)
		}
		if err.Unwrap() != originalErr {
			t.Errorf("Expected Unwrap to return original error")
		}
	})
}

func TestFormatError(t *testing.T) {
	t.Run("raioz error", func(t *testing.T) {
		err := New(ErrCodeInvalidConfig, "test error").
			WithContext("key", "value").
			WithSuggestion("fix it")

		formatted := FormatError(err)
		if formatted == "" {
			t.Error("Expected formatted error, got empty string")
		}
		if !contains(formatted, "INVALID_CONFIG") {
			t.Error("Expected formatted error to contain error code")
		}
		if !contains(formatted, "test error") {
			t.Error("Expected formatted error to contain message")
		}
		if !contains(formatted, "fix it") {
			t.Error("Expected formatted error to contain suggestion")
		}
	})

	t.Run("regular error", func(t *testing.T) {
		err := errors.New("regular error")
		formatted := FormatError(err)
		if formatted == "" {
			t.Error("Expected formatted error, got empty string")
		}
		if !contains(formatted, "regular error") {
			t.Error("Expected formatted error to contain message")
		}
	})
}

func TestAs(t *testing.T) {
	t.Run("raioz error", func(t *testing.T) {
		err := New(ErrCodeInvalidConfig, "test error")
		var target *RaiozError
		if !As(err, &target) {
			t.Error("Expected As to return true for RaiozError")
		}
		if target == nil {
			t.Error("Expected target to be set")
		}
	})

	t.Run("wrapped error", func(t *testing.T) {
		original := New(ErrCodeInvalidConfig, "test error")
		wrapped := &wrappedError{err: original}
		var target *RaiozError
		if !As(wrapped, &target) {
			t.Error("Expected As to return true for wrapped RaiozError")
		}
	})

	t.Run("regular error", func(t *testing.T) {
		err := errors.New("regular error")
		var target *RaiozError
		if As(err, &target) {
			t.Error("Expected As to return false for regular error")
		}
	})
}

// wrappedError is a test helper that wraps an error
type wrappedError struct {
	err error
}

func (e *wrappedError) Error() string {
	return "wrapped: " + e.err.Error()
}

func (e *wrappedError) Unwrap() error {
	return e.err
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr ||
			(len(s) > len(substr) && containsSubstring(s, substr)))
}

func containsSubstring(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
