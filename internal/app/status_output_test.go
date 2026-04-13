package app

import (
	"testing"
	"time"
)

func TestParseHealthCommandOutput(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect bool
	}{
		{"on", "on", true},
		{"off", "off", false},
		{"ON upper", "ON", true},
		{"off with spaces", "  off  ", false},
		{"json active", `{"status":"active"}`, true},
		{"json running", `{"status":"running"}`, true},
		{"json healthy", `{"status":"healthy"}`, true},
		{"json up", `{"status":"up"}`, true},
		{"json inactive", `{"status":"inactive"}`, false},
		{"json stopped", `{"status":"stopped"}`, false},
		{"json unhealthy", `{"status":"unhealthy"}`, false},
		{"json no status", `{"other":"val"}`, true},
		{"plain text", "alive and kicking", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseHealthCommandOutput(tt.input)
			if got != tt.expect {
				t.Errorf("parseHealthCommandOutput(%q)=%v, want %v", tt.input, got, tt.expect)
			}
		})
	}
}

func TestFormatUptimeForStatus(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{5 * time.Minute, "5m"},
		{2 * time.Hour, "2h 0m"},
		{25 * time.Hour, "1d 1h 0m"},
		{48*time.Hour + 30*time.Minute, "2d 0h 30m"},
	}
	for _, tt := range tests {
		got := formatUptimeForStatus(tt.d)
		if got != tt.want {
			t.Errorf("formatUptimeForStatus(%v)=%q, want %q", tt.d, got, tt.want)
		}
	}
}
