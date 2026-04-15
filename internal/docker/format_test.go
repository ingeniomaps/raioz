package docker

import (
	"strings"
	"testing"
)

func TestStripANSI(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"plain string", "hello", "hello"},
		{"colored green", ColorGreen + "ok" + ColorReset, "ok"},
		{"colored red", ColorRed + "fail" + ColorReset, "fail"},
		{"multiple codes", ColorYellow + "a" + ColorReset + ColorBlue + "b" + ColorReset, "ab"},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripANSI(tt.input)
			if got != tt.expected {
				t.Errorf("stripANSI(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestPadString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		width    int
		wantLen  int // length of stripped output
		wantPref string
	}{
		{"shorter than width", "hi", 5, 5, "hi   "},
		{"equal to width", "hello", 5, 5, "hello"},
		{"longer than width", "helloworld", 5, 10, "helloworld"},
		{"with ansi codes", ColorGreen + "ok" + ColorReset, 5, 5, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := padString(tt.input, tt.width)
			stripped := stripANSI(got)
			if len(stripped) != tt.wantLen {
				t.Errorf("padString(%q, %d) stripped len = %d, want %d", tt.input, tt.width, len(stripped), tt.wantLen)
			}
			if tt.wantPref != "" && got != tt.wantPref {
				t.Errorf("padString(%q, %d) = %q, want %q", tt.input, tt.width, got, tt.wantPref)
			}
		})
	}
}

func TestColorizeStatus(t *testing.T) {
	tests := []struct {
		status   string
		contains string
	}{
		{"running", ColorGreen},
		{"stopped", ColorRed},
		{"unknown", ""},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			got := ColorizeStatus(tt.status)
			if tt.contains != "" && !strings.Contains(got, tt.contains) {
				t.Errorf("ColorizeStatus(%q) = %q, should contain %q", tt.status, got, tt.contains)
			}
			if !strings.Contains(stripANSI(got), tt.status) {
				t.Errorf("ColorizeStatus(%q) stripped = %q, should contain status", tt.status, stripANSI(got))
			}
		})
	}
}

func TestColorizeHealth(t *testing.T) {
	tests := []struct {
		health       string
		wantStripped string
	}{
		{"healthy", "healthy"},
		{"unhealthy", "unhealthy"},
		{"starting", "starting"},
		{"none", "n/a"},
		{"other", "other"},
	}

	for _, tt := range tests {
		t.Run(tt.health, func(t *testing.T) {
			got := stripANSI(ColorizeHealth(tt.health))
			if got != tt.wantStripped {
				t.Errorf("ColorizeHealth(%q) stripped = %q, want %q", tt.health, got, tt.wantStripped)
			}
		})
	}
}

func TestFormatDate(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "git commit date",
			input: "2024-05-15 10:30:45 +0200",
			want:  "2024-05-15 10:30",
		},
		{
			name:  "container start",
			input: "2024-05-15 10:30:45",
			want:  "2024-05-15 10:30",
		},
		{
			name:  "unparseable long",
			input: "some-random-string-longer-than-sixteen",
			want:  "some-random-stri",
		},
		{
			name:  "short",
			input: "short",
			want:  "short",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatDate(tt.input)
			if got != tt.want {
				t.Errorf("formatDate(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestFormatStatusTable(t *testing.T) {
	services := map[string]*ServiceInfo{
		"api": {
			Name:    "api",
			Status:  "running",
			Health:  "healthy",
			Uptime:  "5m",
			CPU:     "10%",
			Memory:  "100MB",
			Version: "abc123",
		},
		"linked-svc": {
			Name:       "linked-svc",
			Status:     "stopped",
			Health:     "none",
			Linked:     true,
			LinkTarget: "/some/path",
		},
	}

	// JSON mode is a no-op
	if err := FormatStatusTable(services, true); err != nil {
		t.Errorf("FormatStatusTable(json=true) error = %v", err)
	}

	// Text mode writes to stdout; shouldn't error
	if err := FormatStatusTable(services, false); err != nil {
		t.Errorf("FormatStatusTable(json=false) error = %v", err)
	}
}
