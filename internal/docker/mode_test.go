package docker

import (
	"raioz/internal/config"
	"raioz/internal/workspace"
	"testing"
)

func TestFilterDevVolumes(t *testing.T) {
	tests := []struct {
		name     string
		volumes  []string
		mode     string
		wantLen  int
	}{
		{
			name:    "dev mode keeps all volumes",
			volumes: []string{"mongo-data:/data", "./app:/app", "redis-data:/data"},
			mode:    "dev",
			wantLen: 3,
		},
		{
			name:    "prod mode removes bind mounts",
			volumes: []string{"mongo-data:/data", "./app:/app", "redis-data:/data"},
			mode:    "prod",
			wantLen: 2, // Only named volumes
		},
		{
			name:    "prod mode keeps named volumes",
			volumes: []string{"mongo-data:/data", "redis-data:/data"},
			mode:    "prod",
			wantLen: 2,
		},
		{
			name:    "prod mode removes all bind mounts",
			volumes: []string{"./app:/app", "./config:/config"},
			mode:    "prod",
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filtered := FilterDevVolumes(tt.volumes, tt.mode)
			if len(filtered) != tt.wantLen {
				t.Errorf("FilterDevVolumes() = %d volumes, want %d", len(filtered), tt.wantLen)
			}
		})
	}
}

func TestGetHealthcheckConfig(t *testing.T) {
	// Test prod mode
	prodConfig := GetHealthcheckConfig("prod")
	if prodConfig == nil {
		t.Error("GetHealthcheckConfig() should return config for prod")
	}
	if interval, ok := prodConfig["interval"].(string); !ok || interval != "30s" {
		t.Errorf("GetHealthcheckConfig(prod) interval = %v, want 30s", interval)
	}

	// Test dev mode
	devConfig := GetHealthcheckConfig("dev")
	if devConfig == nil {
		t.Error("GetHealthcheckConfig() should return config for dev")
	}
	if interval, ok := devConfig["interval"].(string); !ok || interval != "60s" {
		t.Errorf("GetHealthcheckConfig(dev) interval = %v, want 60s", interval)
	}
}

func TestGetLoggingConfig(t *testing.T) {
	// Test prod mode
	prodConfig := GetLoggingConfig("prod")
	if prodConfig == nil {
		t.Error("GetLoggingConfig() should return config for prod")
	}
	if driver, ok := prodConfig["driver"].(string); !ok || driver != "json-file" {
		t.Errorf("GetLoggingConfig(prod) driver = %v, want json-file", driver)
	}

	// Test dev mode
	devConfig := GetLoggingConfig("dev")
	if devConfig == nil {
		t.Error("GetLoggingConfig() should return config for dev")
	}
	if driver, ok := devConfig["driver"].(string); !ok || driver != "json-file" {
		t.Errorf("GetLoggingConfig(dev) driver = %v, want json-file", driver)
	}
}

func TestApplyModeConfig(t *testing.T) {
	svc := config.Service{
		Source: config.SourceConfig{
			Kind: "git",
			Path: "test-service",
		},
		Docker: config.DockerConfig{
			Mode:    "dev",
			Volumes: []string{"mongo-data:/data", "./app:/app"},
			Runtime: "node",
		},
	}

	// Create mock workspace
	ws := &workspace.Workspace{
		Root:        "/tmp/test",
		ServicesDir: "/tmp/services",
		EnvDir:      "/tmp/env",
	}

	serviceConfig := map[string]any{
		"ports":   []string{"3000:3000"},
		"volumes": []string{"mongo-data:/data", "./app:/app"},
	}

	// Apply dev mode config
	ApplyModeConfig(serviceConfig, "test-service", svc, ws)

	// Check that volumes still exist (dev mode keeps bind mounts)
	if volumes, ok := serviceConfig["volumes"].([]string); ok {
		if len(volumes) < 2 {
			t.Error("ApplyModeConfig(dev) should keep all volumes")
		}
	} else {
		t.Error("ApplyModeConfig() should preserve volumes in dev mode")
	}

	// Check that logging is added
	if _, ok := serviceConfig["logging"]; !ok {
		t.Error("ApplyModeConfig() should add logging config")
	}

	// Check restart policy
	if restart, ok := serviceConfig["restart"].(string); !ok || restart != "no" {
		t.Errorf("ApplyModeConfig(dev) restart = %v, want no", restart)
	}
}

func TestApplyModeConfigProd(t *testing.T) {
	svc := config.Service{
		Source: config.SourceConfig{
			Kind: "git",
			Path: "test-service",
		},
		Docker: config.DockerConfig{
			Mode:    "prod",
			Volumes: []string{"mongo-data:/data", "./app:/app"},
		},
	}

	ws := &workspace.Workspace{
		Root:        "/tmp/test",
		ServicesDir: "/tmp/services",
		EnvDir:      "/tmp/env",
	}

	serviceConfig := map[string]any{
		"ports":   []string{"3000:3000"},
		"volumes": []string{"mongo-data:/data", "./app:/app"},
	}

	// Apply prod mode config
	ApplyModeConfig(serviceConfig, "test-service", svc, ws)

	// Check that bind mounts are removed in prod
	if volumes, ok := serviceConfig["volumes"].([]string); ok {
		hasBindMount := false
		for _, vol := range volumes {
			if vol == "./app:/app" {
				hasBindMount = true
				break
			}
		}
		if hasBindMount {
			t.Error("ApplyModeConfig(prod) should remove bind mounts")
		}
	}

	// Check that healthcheck is added
	if _, ok := serviceConfig["healthcheck"]; !ok {
		t.Error("ApplyModeConfig(prod) should add healthcheck")
	}

	// Check restart policy
	if restart, ok := serviceConfig["restart"].(string); !ok || restart != "unless-stopped" {
		t.Errorf("ApplyModeConfig(prod) restart = %v, want unless-stopped", restart)
	}
}
