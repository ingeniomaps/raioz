package env

import (
	"testing"
)

func TestIsSensitiveKey(t *testing.T) {
	tests := []struct {
		key      string
		expected bool
	}{
		{"PASSWORD", true},
		{"DATABASE_PASSWORD", true},
		{"API_KEY", true},
		{"AWS_SECRET_ACCESS_KEY", true},
		{"TOKEN", true},
		{"SECRET", true},
		{"PRIVATE_KEY", true},
		{"CREDENTIAL", true},
		{"password", true}, // Case insensitive
		{"Api_Key", true},  // Case insensitive
		{"DATABASE_URL", false},
		{"LOG_LEVEL", false},
		{"NODE_ENV", false},
		{"PORT", false},
		{"DEBUG", false},
		{"API_URL", false},
		{"PASS", false}, // Not a complete match
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			result := isSensitiveKey(tt.key)
			if result != tt.expected {
				t.Errorf("isSensitiveKey(%q) = %v, want %v", tt.key, result, tt.expected)
			}
		})
	}
}

func TestSanitizeEnvValue(t *testing.T) {
	tests := []struct {
		key      string
		value    string
		expected string
	}{
		{"PASSWORD", "secret123", "***REDACTED***"},
		{"DATABASE_PASSWORD", "mypassword", "***REDACTED***"},
		{"API_KEY", "abc123", "***REDACTED***"},
		{"TOKEN", "xyz789", "***REDACTED***"},
		{"DATABASE_URL", "postgres://user:pass@host/db", "postgres://user:pass@host/db"},
		{"LOG_LEVEL", "debug", "debug"},
		{"PORT", "8080", "8080"},
		{"password", "secret", "***REDACTED***"}, // Case insensitive
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			result := SanitizeEnvValue(tt.key, tt.value)
			if result != tt.expected {
				t.Errorf("SanitizeEnvValue(%q, %q) = %q, want %q", tt.key, tt.value, result, tt.expected)
			}
		})
	}
}

func TestSanitizeEnvMap(t *testing.T) {
	env := map[string]string{
		"DATABASE_URL":      "postgres://localhost/db",
		"DATABASE_PASSWORD": "secret123",
		"API_KEY":           "abc123",
		"LOG_LEVEL":         "debug",
		"TOKEN":             "xyz789",
	}

	sanitized := SanitizeEnvMap(env)

	// Check non-sensitive values remain
	if sanitized["DATABASE_URL"] != "postgres://localhost/db" {
		t.Errorf("DATABASE_URL should not be sanitized")
	}
	if sanitized["LOG_LEVEL"] != "debug" {
		t.Errorf("LOG_LEVEL should not be sanitized")
	}

	// Check sensitive values are redacted
	if sanitized["DATABASE_PASSWORD"] != "***REDACTED***" {
		t.Errorf("DATABASE_PASSWORD should be sanitized")
	}
	if sanitized["API_KEY"] != "***REDACTED***" {
		t.Errorf("API_KEY should be sanitized")
	}
	if sanitized["TOKEN"] != "***REDACTED***" {
		t.Errorf("TOKEN should be sanitized")
	}
}

func TestSanitizeEnvString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "single sensitive key",
			input:    "DATABASE_PASSWORD=secret123",
			expected: "DATABASE_PASSWORD=***REDACTED***",
		},
		{
			name:     "multiple keys",
			input:    "DATABASE_URL=postgres://localhost/db\nDATABASE_PASSWORD=secret123\nLOG_LEVEL=debug",
			expected: "DATABASE_URL=postgres://localhost/db\nDATABASE_PASSWORD=***REDACTED***\nLOG_LEVEL=debug",
		},
		{
			name:     "non-sensitive keys",
			input:    "LOG_LEVEL=debug\nPORT=8080\nNODE_ENV=development",
			expected: "LOG_LEVEL=debug\nPORT=8080\nNODE_ENV=development",
		},
		{
			name:     "mixed content",
			input:    "Error: failed to connect\nDATABASE_PASSWORD=secret123\nTry again",
			expected: "Error: failed to connect\nDATABASE_PASSWORD=***REDACTED***\nTry again",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeEnvString(tt.input)
			if result != tt.expected {
				t.Errorf("SanitizeEnvString() = %q, want %q", result, tt.expected)
			}
		})
	}
}
