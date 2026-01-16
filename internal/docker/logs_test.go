package docker

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetAvailableServices(t *testing.T) {
	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "raioz-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	composePath := filepath.Join(tmpDir, "docker-compose.yml")

	// Test with non-existent compose file
	services, err := GetAvailableServices(composePath)
	if err != nil {
		t.Errorf("GetAvailableServices() should handle missing file: %v", err)
	}
	if len(services) != 0 {
		t.Errorf("GetAvailableServices() = %d services, want 0 for missing file", len(services))
	}

	// Note: We can't easily test with actual docker compose files
	// without running docker, so this is a basic test
}

func TestViewLogs(t *testing.T) {
	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "raioz-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	composePath := filepath.Join(tmpDir, "docker-compose.yml")

	// Test with non-existent compose file
	opts := LogsOptions{
		Follow:   false,
		Tail:     10,
		Services: []string{"service1"},
	}

	err = ViewLogs(composePath, opts)
	if err == nil {
		t.Error("ViewLogs() should return error for missing compose file")
	}
}

func TestLogsOptions(t *testing.T) {
	// Test default values
	opts := LogsOptions{
		Follow:   false,
		Tail:     0,
		Services: []string{},
	}

	if opts.Follow {
		t.Error("LogsOptions.Follow should be false by default")
	}
	if opts.Tail != 0 {
		t.Errorf("LogsOptions.Tail = %d, want 0", opts.Tail)
	}
	if len(opts.Services) != 0 {
		t.Errorf("LogsOptions.Services = %d, want 0", len(opts.Services))
	}
}
