package output

import (
	"strings"
	"testing"
	"time"
)

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		want     string
	}{
		{
			name:     "seconds",
			duration: 5 * time.Second,
			want:     "5s",
		},
		{
			name:     "minutes",
			duration: 2*time.Minute + 30*time.Second,
			want:     "2m 30s",
		},
		{
			name:     "hours",
			duration: 1*time.Hour + 15*time.Minute,
			want:     "1h 15m",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatDuration(tt.duration)
			if got != tt.want {
				t.Errorf("FormatDuration() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormatConfigChanges(t *testing.T) {
	tests := []struct {
		name    string
		changes []string
		want    string
	}{
		{
			name:    "empty",
			changes: []string{},
			want:    "",
		},
		{
			name:    "single change",
			changes: []string{"branch changed"},
			want:    "    branch changed\n",
		},
		{
			name:    "multiple changes",
			changes: []string{"branch changed", "port changed"},
			want:    "    branch changed\n    port changed\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatConfigChanges(tt.changes)
			if got != tt.want {
				t.Errorf("FormatConfigChanges() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPrintFunctions(t *testing.T) {
	// These functions just print to stdout, so we can't easily test them
	// without capturing stdout. For now, we just verify they don't panic.
	t.Run("PrintSuccess", func(t *testing.T) {
		PrintSuccess("test message")
	})

	t.Run("PrintWarning", func(t *testing.T) {
		PrintWarning("test warning")
	})

	t.Run("PrintError", func(t *testing.T) {
		PrintError("test error")
	})

	t.Run("PrintInfo", func(t *testing.T) {
		PrintInfo("test info")
	})

	t.Run("PrintServiceCloned", func(t *testing.T) {
		PrintServiceCloned("test-service")
	})

	t.Run("PrintServiceUsingImage", func(t *testing.T) {
		PrintServiceUsingImage("test-service")
	})

	t.Run("PrintInfraStarted", func(t *testing.T) {
		PrintInfraStarted("test-infra")
	})

	t.Run("PrintWorkspaceCreated", func(t *testing.T) {
		PrintWorkspaceCreated()
	})

	t.Run("PrintGeneratingCompose", func(t *testing.T) {
		PrintGeneratingCompose()
	})

	t.Run("PrintStartingServices", func(t *testing.T) {
		PrintStartingServices()
	})

	t.Run("PrintProjectStarted", func(t *testing.T) {
		PrintProjectStarted("test-project")
	})
}

func TestPrintSummary(t *testing.T) {
	services := []string{"service1", "service2"}
	infra := []string{"infra1"}
	duration := 5 * time.Second

	// We can't easily capture stdout, but we can verify it doesn't panic
	PrintSummary(services, infra, duration)
}

func TestPrintSummaryEmpty(t *testing.T) {
	services := []string{}
	infra := []string{}
	duration := 1 * time.Second

	PrintSummary(services, infra, duration)
}

func TestFormatConfigChangesEmpty(t *testing.T) {
	changes := []string{}
	result := FormatConfigChanges(changes)
	if result != "" {
		t.Errorf("FormatConfigChanges([]) = %q, want \"\"", result)
	}
}

func TestFormatConfigChangesContainsExpected(t *testing.T) {
	changes := []string{"change1", "change2"}
	result := FormatConfigChanges(changes)
	if !strings.Contains(result, "change1") {
		t.Error("FormatConfigChanges should contain change1")
	}
	if !strings.Contains(result, "change2") {
		t.Error("FormatConfigChanges should contain change2")
	}
}
