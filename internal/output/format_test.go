package output

import (
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
