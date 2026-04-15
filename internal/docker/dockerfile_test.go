package docker

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"raioz/internal/config"
	"raioz/internal/workspace"
)

func TestValidateDockerfile(t *testing.T) {
	tmpDir := t.TempDir()

	// Test with existing dockerfile
	dockerfilePath := filepath.Join(tmpDir, "Dockerfile.dev")
	if err := os.WriteFile(dockerfilePath, []byte("FROM node:20"), 0644); err != nil {
		t.Fatalf("Failed to create test dockerfile: %v", err)
	}

	exists, err := ValidateDockerfile(tmpDir, "Dockerfile.dev")
	if err != nil {
		t.Fatalf("ValidateDockerfile() error = %v", err)
	}
	if !exists {
		t.Error("ValidateDockerfile() should return true for existing file")
	}

	// Test with non-existing dockerfile
	exists, err = ValidateDockerfile(tmpDir, "Dockerfile.nonexistent")
	if err != nil {
		t.Fatalf("ValidateDockerfile() error = %v", err)
	}
	if exists {
		t.Error("ValidateDockerfile() should return false for non-existing file")
	}
}

func TestGetBaseImageForRuntime(t *testing.T) {
	tests := []struct {
		runtime   string
		wantImage string
	}{
		{"node", "node:22-alpine"},
		{"Node", "node:22-alpine"},
		{"NODEJS", "node:22-alpine"},
		{"go", "golang:1.22-alpine"},
		{"python", "python:3.11-alpine"},
		{"java", "openjdk:17-alpine"},
		{"rust", "rust:1.75-alpine"},
		{"", "node:22-alpine"},        // default
		{"unknown", "node:22-alpine"}, // default
	}

	for _, tt := range tests {
		t.Run(tt.runtime, func(t *testing.T) {
			got := getBaseImageForRuntime(tt.runtime)
			if got != tt.wantImage {
				t.Errorf("getBaseImageForRuntime(%s) = %v, want %v", tt.runtime, got, tt.wantImage)
			}
		})
	}
}

func TestGenerateDockerfileWrapper(t *testing.T) {
	tmpDir := t.TempDir()
	ws := &workspace.Workspace{
		Root:                tmpDir,
		ServicesDir:         tmpDir,
		LocalServicesDir:    tmpDir,
		ReadonlyServicesDir: tmpDir,
		EnvDir:              tmpDir,
	}

	svc := config.Service{
		Source: config.SourceConfig{
			Kind: "git",
			Path: "test-service",
		},
		Docker: &config.DockerConfig{
			Command: "npm run dev",
			Runtime: "node",
		},
	}

	wrapperPath, err := GenerateDockerfileWrapper(ws, "test-service", svc)
	if err != nil {
		t.Fatalf("GenerateDockerfileWrapper() error = %v", err)
	}

	if wrapperPath == "" {
		t.Error("GenerateDockerfileWrapper() should return a path")
	}

	// Verify file exists
	if _, err := os.Stat(wrapperPath); os.IsNotExist(err) {
		t.Errorf("Wrapper dockerfile does not exist: %v", wrapperPath)
	}

	// Verify content
	content, err := os.ReadFile(wrapperPath)
	if err != nil {
		t.Fatalf("Failed to read wrapper dockerfile: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "FROM node:22-alpine") {
		t.Error("Wrapper should contain base image")
	}
	if !strings.Contains(contentStr, "npm run dev") {
		t.Error("Wrapper should contain command")
	}
}

func TestEnsureDockerfile(t *testing.T) {
	tmpDir := t.TempDir()
	servicePath := filepath.Join(tmpDir, "test-service")
	if err := os.MkdirAll(servicePath, 0755); err != nil {
		t.Fatalf("Failed to create service directory: %v", err)
	}

	ws := &workspace.Workspace{
		Root:                tmpDir,
		ServicesDir:         tmpDir,
		LocalServicesDir:    tmpDir,
		ReadonlyServicesDir: tmpDir,
		EnvDir:              tmpDir,
	}

	// Test 1: Dockerfile exists
	dockerfilePath := filepath.Join(servicePath, "Dockerfile.dev")
	if err := os.WriteFile(dockerfilePath, []byte("FROM node:20"), 0644); err != nil {
		t.Fatalf("Failed to create dockerfile: %v", err)
	}

	svc := config.Service{
		Source: config.SourceConfig{
			Kind: "git",
			Path: "test-service",
		},
		Docker: &config.DockerConfig{
			Dockerfile: "Dockerfile.dev",
		},
	}

	path, err := EnsureDockerfile(ws, "test-service", svc)
	if err != nil {
		t.Fatalf("EnsureDockerfile() error = %v", err)
	}
	if path != "Dockerfile.dev" {
		t.Errorf("EnsureDockerfile() = %v, want Dockerfile.dev", path)
	}

	// Test 2: Dockerfile doesn't exist, no command
	svc2 := config.Service{
		Source: config.SourceConfig{
			Kind: "git",
			Path: "test-service-2",
		},
		Docker: &config.DockerConfig{
			Dockerfile: "Dockerfile.dev",
		},
	}

	_, err = EnsureDockerfile(ws, "test-service-2", svc2)
	if err == nil {
		t.Error("EnsureDockerfile() should error when dockerfile missing and no command")
	}

	// Test 3: Dockerfile doesn't exist, but command provided
	svc3 := config.Service{
		Source: config.SourceConfig{
			Kind: "git",
			Path: "test-service-3",
		},
		Docker: &config.DockerConfig{
			Dockerfile: "Dockerfile.dev",
			Command:    "npm run dev",
			Runtime:    "node",
		},
	}

	path, err = EnsureDockerfile(ws, "test-service-3", svc3)
	if err != nil {
		t.Fatalf("EnsureDockerfile() error = %v", err)
	}
	if path == "" {
		t.Error("EnsureDockerfile() should return wrapper path when dockerfile missing but command provided")
	}
}
