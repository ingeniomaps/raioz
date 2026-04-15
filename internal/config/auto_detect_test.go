package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAutoDetect_BasicProject(t *testing.T) {
	dir := t.TempDir()

	// Create a Go API service
	apiDir := filepath.Join(dir, "api")
	os.MkdirAll(apiDir, 0755)
	os.WriteFile(filepath.Join(apiDir, "go.mod"), []byte("module api\ngo 1.22\n"), 0644)
	os.WriteFile(filepath.Join(apiDir, ".env"), []byte("DATABASE_URL=postgres://localhost:5432/db\n"), 0644)

	// Create a Node frontend
	frontDir := filepath.Join(dir, "frontend")
	os.MkdirAll(frontDir, 0755)
	os.WriteFile(filepath.Join(frontDir, "package.json"), []byte(`{"scripts":{"dev":"next dev"}}`), 0644)

	deps, err := AutoDetect(dir)
	if err != nil {
		t.Fatalf("AutoDetect failed: %v", err)
	}

	if deps.SchemaVersion != "2.0" {
		t.Errorf("schema = %q, want '2.0'", deps.SchemaVersion)
	}

	// Should detect both services
	if len(deps.Services) != 2 {
		t.Errorf("services = %d, want 2", len(deps.Services))
	}
	if _, ok := deps.Services["api"]; !ok {
		t.Error("expected 'api' service")
	}
	if _, ok := deps.Services["frontend"]; !ok {
		t.Error("expected 'frontend' service")
	}

	// Should infer postgres from .env
	if len(deps.Infra) == 0 {
		t.Error("expected at least 1 infra dependency (postgres)")
	}
	if _, ok := deps.Infra["postgres"]; !ok {
		t.Error("expected 'postgres' infra inferred from DATABASE_URL")
	}

	// api should depend on postgres
	apiSvc := deps.Services["api"]
	found := false
	for _, dep := range apiSvc.DependsOn {
		if dep == "postgres" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected api dependsOn postgres, got %v", apiSvc.DependsOn)
	}
}

func TestAutoDetect_SingleRootProject(t *testing.T) {
	dir := t.TempDir()

	// Just a go.mod in root, no subdirectories
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module myapp\ngo 1.22\n"), 0644)

	deps, err := AutoDetect(dir)
	if err != nil {
		t.Fatalf("AutoDetect failed: %v", err)
	}

	if len(deps.Services) != 1 {
		t.Errorf("services = %d, want 1 (root project)", len(deps.Services))
	}
}

func TestAutoDetect_EmptyDir(t *testing.T) {
	dir := t.TempDir()

	_, err := AutoDetect(dir)
	if err == nil {
		t.Error("expected error for empty directory")
	}
}

func TestAutoDetect_IgnoredDirs(t *testing.T) {
	dir := t.TempDir()

	// Create a service
	apiDir := filepath.Join(dir, "api")
	os.MkdirAll(apiDir, 0755)
	os.WriteFile(filepath.Join(apiDir, "go.mod"), []byte("module api\ngo 1.22\n"), 0644)

	// Create ignored directories (should not be scanned)
	os.MkdirAll(filepath.Join(dir, "node_modules", "fake"), 0755)
	os.WriteFile(filepath.Join(dir, "node_modules", "fake", "package.json"), []byte(`{}`), 0644)
	os.MkdirAll(filepath.Join(dir, "vendor"), 0755)
	os.WriteFile(filepath.Join(dir, "vendor", "go.mod"), []byte("module vendor\n"), 0644)

	deps, err := AutoDetect(dir)
	if err != nil {
		t.Fatalf("AutoDetect failed: %v", err)
	}

	// Should only find api, not node_modules or vendor
	if len(deps.Services) != 1 {
		t.Errorf("services = %d, want 1 (only api)", len(deps.Services))
	}
	if _, ok := deps.Services["api"]; !ok {
		t.Error("expected 'api' service")
	}
}

func TestAutoDetect_MultipleEnvDeps(t *testing.T) {
	dir := t.TempDir()

	apiDir := filepath.Join(dir, "api")
	os.MkdirAll(apiDir, 0755)
	os.WriteFile(filepath.Join(apiDir, "go.mod"), []byte("module api\ngo 1.22\n"), 0644)
	os.WriteFile(filepath.Join(apiDir, ".env"), []byte(
		"DATABASE_URL=postgres://localhost:5432/db\n"+
			"REDIS_URL=redis://localhost:6379\n"+
			"RABBITMQ_URL=amqp://localhost:5672\n",
	), 0644)

	deps, err := AutoDetect(dir)
	if err != nil {
		t.Fatalf("AutoDetect failed: %v", err)
	}

	if len(deps.Infra) < 3 {
		t.Errorf("infra = %d, want >= 3 (postgres, redis, rabbitmq)", len(deps.Infra))
	}

	for _, expected := range []string{"postgres", "redis", "rabbitmq"} {
		if _, ok := deps.Infra[expected]; !ok {
			t.Errorf("expected '%s' infra dependency", expected)
		}
	}
}
