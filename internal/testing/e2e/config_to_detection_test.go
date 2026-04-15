package e2e

import (
	"os"
	"path/filepath"
	"testing"

	"raioz/internal/config"
	"raioz/internal/detect"
)

// TestFullFlow_YAMLToDetection verifies the complete chain:
// raioz.yaml → LoadYAML → YAMLToDeps → detect runtimes → correct results
func TestFullFlow_YAMLToDetection(t *testing.T) {
	// Create a realistic project structure
	dir := t.TempDir()

	// Service: Go API with Dockerfile
	apiDir := filepath.Join(dir, "api")
	os.MkdirAll(apiDir, 0755)
	os.WriteFile(filepath.Join(apiDir, "go.mod"), []byte("module example.com/api\n\ngo 1.22\n"), 0644)
	os.WriteFile(filepath.Join(apiDir, "Dockerfile"), []byte("FROM golang:1.22\nCOPY . .\nCMD [\"go\", \"run\", \".\"]\n"), 0644)
	os.WriteFile(filepath.Join(apiDir, ".env"), []byte("DATABASE_URL=postgres://localhost:5432/api\nREDIS_URL=redis://localhost:6379\n"), 0644)

	// Service: Next.js frontend
	frontDir := filepath.Join(dir, "frontend")
	os.MkdirAll(frontDir, 0755)
	os.WriteFile(filepath.Join(frontDir, "package.json"), []byte(`{"scripts":{"dev":"next dev","start":"next start"}}`), 0644)

	// Service: Worker with docker-compose
	workerDir := filepath.Join(dir, "worker")
	os.MkdirAll(workerDir, 0755)
	os.WriteFile(filepath.Join(workerDir, "docker-compose.yml"), []byte("version: '3'\nservices:\n  worker:\n    build: .\n"), 0644)

	// raioz.yaml
	yamlContent := `
project: my-platform
workspace: acme

services:
  api:
    path: ./api
    dependsOn: [postgres, redis]
    health: /health

  frontend:
    path: ./frontend
    dependsOn: [api]
    watch: native

  worker:
    path: ./worker
    dependsOn: [postgres]

dependencies:
  postgres:
    image: postgres:16
    ports: ["5432"]

  redis:
    image: redis:7
    ports: ["6379"]
`
	configPath := filepath.Join(dir, "raioz.yaml")
	os.WriteFile(configPath, []byte(yamlContent), 0644)

	// Step 1: Load YAML
	cfg, err := config.LoadYAML(configPath)
	if err != nil {
		t.Fatalf("LoadYAML failed: %v", err)
	}

	if cfg.Project != "my-platform" {
		t.Errorf("project = %q, want 'my-platform'", cfg.Project)
	}
	if cfg.Workspace != "acme" {
		t.Errorf("workspace = %q, want 'acme'", cfg.Workspace)
	}
	if len(cfg.Services) != 3 {
		t.Errorf("services count = %d, want 3", len(cfg.Services))
	}
	if len(cfg.Deps) != 2 {
		t.Errorf("deps count = %d, want 2", len(cfg.Deps))
	}

	// Step 2: Convert to Deps
	deps, err := config.YAMLToDeps(cfg)
	if err != nil {
		t.Fatalf("YAMLToDeps failed: %v", err)
	}

	if deps.SchemaVersion != "2.0" {
		t.Errorf("schema version = %q, want '2.0'", deps.SchemaVersion)
	}
	if deps.Network.Name != "acme-net" {
		t.Errorf("network = %q, want 'acme-net'", deps.Network.Name)
	}

	// Step 3: Detect runtimes for each service
	apiDetect := detect.Detect(deps.Services["api"].Source.Path)
	if apiDetect.Runtime != detect.RuntimeDockerfile {
		t.Errorf("api runtime = %s, want dockerfile (Dockerfile takes priority over go.mod)", apiDetect.Runtime)
	}

	frontDetect := detect.Detect(deps.Services["frontend"].Source.Path)
	if frontDetect.Runtime != detect.RuntimeNPM {
		t.Errorf("frontend runtime = %s, want npm", frontDetect.Runtime)
	}
	if !frontDetect.HasHotReload {
		t.Error("frontend (next.js) should have hot-reload")
	}

	workerDetect := detect.Detect(deps.Services["worker"].Source.Path)
	if workerDetect.Runtime != detect.RuntimeCompose {
		t.Errorf("worker runtime = %s, want compose", workerDetect.Runtime)
	}

	// Step 4: Verify dependency structure
	apiSvc := deps.Services["api"]
	if len(apiSvc.DependsOn) != 2 {
		t.Errorf("api dependsOn count = %d, want 2", len(apiSvc.DependsOn))
	}

	pgInfra, ok := deps.Infra["postgres"]
	if !ok || pgInfra.Inline == nil {
		t.Fatal("postgres infra not found or not inline")
	}
	if pgInfra.Inline.Image != "postgres" || pgInfra.Inline.Tag != "16" {
		t.Errorf("postgres = %s:%s, want postgres:16", pgInfra.Inline.Image, pgInfra.Inline.Tag)
	}
}

// TestFullFlow_InitScan verifies raioz init scans and generates correct config.
func TestFullFlow_InitScan(t *testing.T) {
	dir := t.TempDir()

	// Go API service with .env
	apiDir := filepath.Join(dir, "api")
	os.MkdirAll(apiDir, 0755)
	os.WriteFile(filepath.Join(apiDir, "go.mod"), []byte("module api\ngo 1.22\n"), 0644)
	os.WriteFile(filepath.Join(apiDir, ".env"), []byte("DATABASE_URL=postgres://pg:5432/db\nREDIS_URL=redis://redis:6379\n"), 0644)

	// Node frontend
	frontDir := filepath.Join(dir, "frontend")
	os.MkdirAll(frontDir, 0755)
	os.WriteFile(filepath.Join(frontDir, "package.json"), []byte(`{"scripts":{"dev":"vite"}}`), 0644)

	// Infer deps from env
	deps, links := detect.InferDepsFromEnv(dir)

	// Should find postgres and redis
	depNames := make(map[string]bool)
	for _, dep := range deps {
		depNames[dep.Name] = true
	}
	if !depNames["postgres"] {
		t.Error("expected postgres inferred from DATABASE_URL")
	}
	if !depNames["redis"] {
		t.Error("expected redis inferred from REDIS_URL")
	}

	// Should have links from api to postgres and redis
	linkMap := make(map[string][]string)
	for _, link := range links {
		linkMap[link.From] = append(linkMap[link.From], link.To)
	}
	if apiLinks, ok := linkMap["api"]; !ok || len(apiLinks) < 2 {
		t.Errorf("expected api → [postgres, redis] links, got %v", linkMap["api"])
	}
}

// TestFullFlow_BackwardCompat verifies .raioz.json still loads correctly.
func TestFullFlow_BackwardCompat(t *testing.T) {
	dir := t.TempDir()
	jsonContent := `{
		"schemaVersion": "1.0",
		"workspace": "legacy",
		"project": {"name": "old-project"},
		"services": {
			"api": {
				"source": {"kind": "local", "path": "."},
				"dependsOn": ["db"]
			}
		},
		"infra": {
			"db": {"image": "postgres", "tag": "15", "ports": ["5432"]}
		},
		"env": {"useGlobal": false, "files": []}
	}`
	configPath := filepath.Join(dir, ".raioz.json")
	os.WriteFile(configPath, []byte(jsonContent), 0644)

	// Should load via the JSON loader
	deps, _, err := config.LoadDeps(configPath)
	if err != nil {
		t.Fatalf("LoadDeps failed for .raioz.json: %v", err)
	}
	if deps.Project.Name != "old-project" {
		t.Errorf("project = %q, want 'old-project'", deps.Project.Name)
	}
	if deps.Workspace != "legacy" {
		t.Errorf("workspace = %q, want 'legacy'", deps.Workspace)
	}

	// IsYAMLConfig should return false
	if config.IsYAMLConfig(configPath) {
		t.Error(".raioz.json should not be detected as YAML")
	}
}
