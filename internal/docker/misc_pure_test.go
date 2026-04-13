package docker

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestExpandTilde(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("home dir: %v", err)
	}

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{"empty", "", "", false},
		{"no tilde", "/abs/path", "/abs/path", false},
		{"just tilde", "~", home, false},
		{"tilde slash", "~/foo", filepath.Join(home, "foo"), false},
		{"tilde user", "~other", "~other", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := expandTilde(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("expandTilde(%q) err = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("expandTilde(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestResolveRelativeVolumes_WithTilde(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("home: %v", err)
	}
	got, err := ResolveRelativeVolumes([]string{"~/data:/app/data"}, "/tmp/proj")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %v", got)
	}
	expected := filepath.Join(home, "data") + ":/app/data"
	if got[0] != expected {
		t.Errorf("got %q, want %q", got[0], expected)
	}
}

func TestResolveRelativeVolumes_WithRoMode(t *testing.T) {
	got, err := ResolveRelativeVolumes([]string{"./data:/app/data:ro"}, "/tmp/proj")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %v", got)
	}
	if !strings.HasSuffix(got[0], ":ro") {
		t.Errorf("expected :ro suffix, got %q", got[0])
	}
}

func TestNormalizeNetworkName(t *testing.T) {
	tests := []struct {
		input   string
		wantErr bool
	}{
		{"my-network", false},
		{"My_Network", false},
		{"", true},
		{"MY NETWORK", false},
		{strings.Repeat("a", 100), false},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := NormalizeNetworkName(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("NormalizeNetworkName(%q) err=%v wantErr=%v", tt.input, err, tt.wantErr)
			}
			if !tt.wantErr && len(got) > MaxNetworkNameLength {
				t.Errorf("result too long: %d", len(got))
			}
		})
	}
}

func TestValidateNetworkName(t *testing.T) {
	if err := ValidateNetworkName("valid-net"); err != nil {
		t.Errorf("valid-net should pass: %v", err)
	}
	if err := ValidateNetworkName(""); err == nil {
		t.Error("empty should fail")
	}
	if err := ValidateNetworkName("INVALID"); err == nil {
		t.Error("uppercase should fail")
	}
}

func TestValidateVolumeName(t *testing.T) {
	if err := ValidateVolumeName("valid-vol"); err != nil {
		t.Errorf("valid-vol should pass: %v", err)
	}
	if err := ValidateVolumeName(""); err == nil {
		t.Error("empty should fail")
	}
	// Very long name should fail
	if err := ValidateVolumeName(strings.Repeat("a", MaxVolumeNameLength+1)); err == nil {
		t.Error("overlong volume should fail")
	}
}

// --- formatUptime ---

func TestFormatUptimeDurations(t *testing.T) {
	tests := []struct {
		name string
		d    time.Duration
		want string
	}{
		{"30 mins", 30 * time.Minute, "30m"},
		{"2 hours 5 mins", 2*time.Hour + 5*time.Minute, "2h 5m"},
		{"3 days 4 hours", 3*24*time.Hour + 4*time.Hour + 10*time.Minute, "3d 4h 10m"},
		{"zero", 0, "0m"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatUptime(tt.d)
			if got != tt.want {
				t.Errorf("formatUptime(%v) = %q, want %q", tt.d, got, tt.want)
			}
		})
	}
}

// --- FormatPortConflicts ---

func TestFormatPortConflicts(t *testing.T) {
	// Empty
	got := FormatPortConflicts(nil)
	if got != "" {
		t.Errorf("empty = %q, want empty", got)
	}

	// Single conflict with alternative
	conflicts := []PortConflict{
		{Port: "3000", Project: "other", Service: "api", Alternative: "3001"},
	}
	got = FormatPortConflicts(conflicts)
	if !strings.Contains(got, "3000") {
		t.Errorf("missing port: %q", got)
	}
	if !strings.Contains(got, "other") {
		t.Errorf("missing project: %q", got)
	}
	if !strings.Contains(got, "3001") {
		t.Errorf("missing alternative: %q", got)
	}

	// Conflict without alternative
	conflicts2 := []PortConflict{
		{Port: "80", Project: "web", Service: "nginx"},
	}
	got2 := FormatPortConflicts(conflicts2)
	if strings.Contains(got2, "alternative") {
		t.Errorf("should not have alternative suggestion: %q", got2)
	}
}

// --- GetProjectUsingPort ---

func TestGetProjectUsingPort_NoActivePorts(t *testing.T) {
	tmpDir := t.TempDir()
	got, err := GetProjectUsingPort("3000", tmpDir)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil for empty dir, got %v", got)
	}
}

func TestGetProjectUsingPort_BadPort(t *testing.T) {
	_, err := GetProjectUsingPort("invalid", "/tmp")
	if err == nil {
		t.Error("expected error for bad port")
	}
}

// --- CheckPortInUse ---

func TestCheckPortInUse(t *testing.T) {
	// Invalid format
	if _, err := CheckPortInUse(""); err == nil {
		t.Error("expected error for empty port")
	}
	if _, err := CheckPortInUse("notanum"); err == nil {
		t.Error("expected error for non-numeric port")
	}
	// Valid high port unlikely to be bound
	inUse, err := CheckPortInUse("0")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	_ = inUse
}
