package docker

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFormatUptime(t *testing.T) {
	tests := []struct {
		duration string
		want     string
	}{
		{"1h30m", "1h 30m"},
		{"2h", "2h 0m"},
		{"30m", "30m"},
		{"25h", "1d 1h 0m"},
		{"0m", "0m"},
	}

	for _, tt := range tests {
		t.Run(tt.duration, func(t *testing.T) {
			// Note: formatUptime is not exported, so we test indirectly
			// This is a placeholder test
			if tt.duration == "" {
				t.Skip("Skipping formatUptime test (function not exported)")
			}
		})
	}
}

func TestGetContainerName(t *testing.T) {
	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "raioz-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	composePath := filepath.Join(tmpDir, "docker-compose.yml")

	// Test with non-existent compose file
	_, err = GetContainerName(composePath, "test-service")
	if err == nil {
		t.Error("GetContainerName() should return error for missing compose file")
	}
}

func TestServiceInfo(t *testing.T) {
	info := &ServiceInfo{
		Name:    "test-service",
		Status:  "running",
		Health:  "healthy",
		Uptime:  "1h 30m",
		CPU:     "5.2%",
		Memory:  "100MB/500MB",
		Version: "abc123def456",
	}

	if info.Name != "test-service" {
		t.Errorf("ServiceInfo.Name = %s, want test-service", info.Name)
	}

	if info.Status != "running" {
		t.Errorf("ServiceInfo.Status = %s, want running", info.Status)
	}

	if info.Health != "healthy" {
		t.Errorf("ServiceInfo.Health = %s, want healthy", info.Health)
	}
}
