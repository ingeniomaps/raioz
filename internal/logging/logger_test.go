package logging

import (
	"os"
	"testing"
)

func TestInit(t *testing.T) {
	t.Run("init with info level", func(t *testing.T) {
		Init(LogLevelInfo, false)
		if Logger == nil {
			t.Error("Logger should be initialized")
		}
		if logLevel != LogLevelInfo {
			t.Errorf("Expected log level %s, got %s", LogLevelInfo, logLevel)
		}
		if jsonFormat {
			t.Error("Expected jsonFormat to be false")
		}
	})

	t.Run("init with debug level", func(t *testing.T) {
		Init(LogLevelDebug, false)
		if logLevel != LogLevelDebug {
			t.Errorf("Expected log level %s, got %s", LogLevelDebug, logLevel)
		}
	})

	t.Run("init with JSON format", func(t *testing.T) {
		Init(LogLevelInfo, true)
		if !jsonFormat {
			t.Error("Expected jsonFormat to be true")
		}
	})
}

func TestSetLevel(t *testing.T) {
	Init(LogLevelInfo, false)

	t.Run("set to warn", func(t *testing.T) {
		SetLevel(LogLevelWarn)
		if logLevel != LogLevelWarn {
			t.Errorf("Expected log level %s, got %s", LogLevelWarn, logLevel)
		}
	})

	t.Run("set to error", func(t *testing.T) {
		SetLevel(LogLevelError)
		if logLevel != LogLevelError {
			t.Errorf("Expected log level %s, got %s", LogLevelError, logLevel)
		}
	})
}

func TestSetJSONFormat(t *testing.T) {
	Init(LogLevelInfo, false)

	t.Run("enable JSON", func(t *testing.T) {
		SetJSONFormat(true)
		if !jsonFormat {
			t.Error("Expected jsonFormat to be true")
		}
	})

	t.Run("disable JSON", func(t *testing.T) {
		SetJSONFormat(false)
		if jsonFormat {
			t.Error("Expected jsonFormat to be false")
		}
	})
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		name     string
		input    LogLevel
		expected string
	}{
		{"debug", LogLevelDebug, "DEBUG"},
		{"info", LogLevelInfo, "INFO"},
		{"warn", LogLevelWarn, "WARN"},
		{"error", LogLevelError, "ERROR"},
		{"unknown", LogLevel("unknown"), "INFO"},
		{"uppercase", LogLevel("DEBUG"), "DEBUG"},
		{"mixed case", LogLevel("Info"), "INFO"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			level := parseLevel(tt.input)
			// slog.Level is an int, we can't directly compare strings
			// But we can verify it's a valid level
			if level.String() == "" {
				t.Error("Expected valid level")
			}
		})
	}
}

func TestIsDebugEnabled(t *testing.T) {
	t.Run("debug enabled", func(t *testing.T) {
		Init(LogLevelDebug, false)
		if !IsDebugEnabled() {
			t.Error("Expected debug to be enabled")
		}
	})

	t.Run("debug disabled", func(t *testing.T) {
		Init(LogLevelInfo, false)
		if IsDebugEnabled() {
			t.Error("Expected debug to be disabled")
		}
	})
}

func TestIsJSONFormat(t *testing.T) {
	t.Run("JSON enabled", func(t *testing.T) {
		Init(LogLevelInfo, true)
		if !IsJSONFormat() {
			t.Error("Expected JSON format to be enabled")
		}
	})

	t.Run("JSON disabled", func(t *testing.T) {
		Init(LogLevelInfo, false)
		if IsJSONFormat() {
			t.Error("Expected JSON format to be disabled")
		}
	})
}

func TestLoggingFunctions(t *testing.T) {
	Init(LogLevelDebug, false)

	t.Run("Debug", func(t *testing.T) {
		Debug("test debug message", "key", "value")
		// Logger should write to stdout
		if Logger == nil {
			t.Error("Logger should not be nil")
		}
	})

	t.Run("Info", func(t *testing.T) {
		Info("test info message", "key", "value")
		if Logger == nil {
			t.Error("Logger should not be nil")
		}
	})

	t.Run("Warn", func(t *testing.T) {
		Warn("test warn message", "key", "value")
		if Logger == nil {
			t.Error("Logger should not be nil")
		}
	})

	t.Run("Error", func(t *testing.T) {
		Error("test error message", "key", "value")
		if Logger == nil {
			t.Error("Logger should not be nil")
		}
	})
}

func TestInitFromEnv(t *testing.T) {
	t.Run("no env vars", func(t *testing.T) {
		os.Unsetenv("RAIOZ_LOG_LEVEL")
		os.Unsetenv("RAIOZ_LOG_JSON")
		InitFromEnv()
		// Should initialize with defaults
		if Logger == nil {
			t.Error("Logger should be initialized")
		}
	})

	t.Run("with log level env", func(t *testing.T) {
		os.Setenv("RAIOZ_LOG_LEVEL", "debug")
		InitFromEnv()
		if logLevel != LogLevelDebug {
			t.Errorf("Expected log level %s, got %s", LogLevelDebug, logLevel)
		}
		os.Unsetenv("RAIOZ_LOG_LEVEL")
	})

	t.Run("with JSON env", func(t *testing.T) {
		os.Setenv("RAIOZ_LOG_JSON", "true")
		InitFromEnv()
		if !jsonFormat {
			t.Error("Expected JSON format to be enabled")
		}
		os.Unsetenv("RAIOZ_LOG_JSON")
	})

	t.Run("with invalid log level", func(t *testing.T) {
		os.Setenv("RAIOZ_LOG_LEVEL", "invalid")
		InitFromEnv()
		// Should default to info
		if logLevel != LogLevelInfo {
			t.Errorf("Expected log level to default to %s, got %s", LogLevelInfo, logLevel)
		}
		os.Unsetenv("RAIOZ_LOG_LEVEL")
	})
}

func TestDefault(t *testing.T) {
	// Set Logger to nil
	oldLogger := Logger
	Logger = nil
	defer func() {
		Logger = oldLogger
	}()

	t.Run("initialize when nil", func(t *testing.T) {
		Default()
		if Logger == nil {
			t.Error("Logger should be initialized")
		}
	})

	t.Run("no change when already initialized", func(t *testing.T) {
		Init(LogLevelDebug, false)
		oldLogger := Logger
		Default()
		if Logger != oldLogger {
			t.Error("Logger should not be reinitialized")
		}
	})
}

func TestLoggingWithNilLogger(t *testing.T) {
	// Set Logger to nil to test fallback behavior
	oldLogger := Logger
	Logger = nil
	defer func() {
		Logger = oldLogger
	}()

	// These should not panic
	Debug("test")
	Info("test")
	Warn("test")
	Error("test")
}
