package cli

import (
	"os"
	"path/filepath"
	"testing"

	"raioz/internal/config"
)

func TestMigrateYAMLCmd(t *testing.T) {
	if migrateYAMLCmd == nil {
		t.Fatal("migrateYAMLCmd should be initialized")
	}
	if migrateYAMLCmd.Use != "yaml" {
		t.Errorf("Use = %s, want yaml", migrateYAMLCmd.Use)
	}
	if migrateYAMLCmd.Short == "" {
		t.Error("Short should not be empty")
	}
}

func TestMigrateYAMLFlags(t *testing.T) {
	flags := []struct {
		name      string
		shorthand string
	}{
		{"from", ""},
		{"output", "o"},
	}
	for _, tt := range flags {
		t.Run(tt.name, func(t *testing.T) {
			f := migrateYAMLCmd.Flags().Lookup(tt.name)
			if f == nil {
				t.Fatalf("flag %q not registered", tt.name)
			}
			if tt.shorthand != "" && f.Shorthand != tt.shorthand {
				t.Errorf("shorthand = %s, want %s", f.Shorthand, tt.shorthand)
			}
		})
	}
}

func TestMigrateYAMLRegisteredUnderMigrate(t *testing.T) {
	found := false
	for _, sub := range migrateCmd.Commands() {
		if sub.Name() == "yaml" {
			found = true
			break
		}
	}
	if !found {
		t.Error("migrateYAMLCmd not registered under migrateCmd")
	}
}

func TestDepsToYAMLConfig(t *testing.T) {
	enabled := true
	deps := &config.Deps{
		Workspace: "acme",
		Project:   config.Project{Name: "ecom"},
		Services: map[string]config.Service{
			"api": {
				Source: config.SourceConfig{
					Kind: "local",
					Path: "./api",
				},
				DependsOn: []string{"postgres"},
				Docker: &config.DockerConfig{
					Ports: []string{"3000:3000"},
				},
				Enabled: &enabled,
			},
			"web": {
				Source: config.SourceConfig{
					Kind:   "git",
					Repo:   "git@github.com:acme/web.git",
					Branch: "main",
					Path:   "./web",
				},
			},
		},
		Infra: map[string]config.InfraEntry{
			"postgres": {
				Inline: &config.Infra{
					Image:   "postgres",
					Tag:     "16",
					Ports:   []string{"5432:5432"},
					Volumes: []string{"pg-data:/var/lib/postgresql/data"},
				},
			},
			// Entry without Inline should be skipped.
			"other": {Path: "other.yaml"},
		},
	}

	cfg := depsToYAMLConfig(deps)

	if cfg.Project != "ecom" {
		t.Errorf("Project = %q, want ecom", cfg.Project)
	}
	if cfg.Workspace != "acme" {
		t.Errorf("Workspace = %q, want acme", cfg.Workspace)
	}
	if len(cfg.Services) != 2 {
		t.Errorf("Services len = %d, want 2", len(cfg.Services))
	}
	apiSvc, ok := cfg.Services["api"]
	if !ok {
		t.Fatal("api service missing in YAML config")
	}
	if apiSvc.Path != "./api" {
		t.Errorf("api.Path = %q, want ./api", apiSvc.Path)
	}
	if len(apiSvc.Ports) != 1 || apiSvc.Ports[0] != "3000:3000" {
		t.Errorf("api.Ports = %v, want [3000:3000]", apiSvc.Ports)
	}
	if len(apiSvc.DependsOn) != 1 || apiSvc.DependsOn[0] != "postgres" {
		t.Errorf("api.DependsOn = %v", apiSvc.DependsOn)
	}
	webSvc, ok := cfg.Services["web"]
	if !ok {
		t.Fatal("web service missing in YAML config")
	}
	if webSvc.Git != "git@github.com:acme/web.git" {
		t.Errorf("web.Git = %q", webSvc.Git)
	}
	if webSvc.Branch != "main" {
		t.Errorf("web.Branch = %q, want main", webSvc.Branch)
	}
	if webSvc.Path != "./web" {
		t.Errorf("web.Path = %q, want ./web", webSvc.Path)
	}

	// Deps: only the inline infra should be translated.
	if len(cfg.Deps) != 1 {
		t.Errorf("Deps len = %d, want 1", len(cfg.Deps))
	}
	pg, ok := cfg.Deps["postgres"]
	if !ok {
		t.Fatal("postgres dep missing in YAML config")
	}
	if pg.Image != "postgres:16" {
		t.Errorf("postgres.Image = %q, want postgres:16", pg.Image)
	}
	if len(pg.Ports) != 1 || pg.Ports[0] != "5432:5432" {
		t.Errorf("postgres.Ports = %v", pg.Ports)
	}
	if len(pg.Volumes) != 1 {
		t.Errorf("postgres.Volumes = %v", pg.Volumes)
	}
}

func TestDepsToYAMLConfigImageWithoutTag(t *testing.T) {
	deps := &config.Deps{
		Project: config.Project{Name: "p"},
		Infra: map[string]config.InfraEntry{
			"redis": {Inline: &config.Infra{Image: "redis"}},
		},
	}
	cfg := depsToYAMLConfig(deps)
	r, ok := cfg.Deps["redis"]
	if !ok {
		t.Fatal("redis missing")
	}
	if r.Image != "redis" {
		t.Errorf("Image = %q, want redis", r.Image)
	}
}

func TestMigrateYAMLRunMissingInput(t *testing.T) {
	dir := t.TempDir()
	origWd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origWd) }()

	origFrom := migrateYAMLFrom
	origOut := migrateYAMLOutput
	migrateYAMLFrom = filepath.Join(dir, "does-not-exist.json")
	migrateYAMLOutput = filepath.Join(dir, "raioz.yaml")
	defer func() {
		migrateYAMLFrom = origFrom
		migrateYAMLOutput = origOut
	}()

	err := migrateYAMLCmd.RunE(migrateYAMLCmd, []string{})
	if err == nil {
		t.Error("expected error loading missing input, got nil")
	}
}

func TestMigrateYAMLRunSuccess(t *testing.T) {
	dir := t.TempDir()
	origWd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origWd) }()

	// Write a minimal valid .raioz.json
	jsonContent := `{
  "schemaVersion": "1.0",
  "project": {"name": "demo"},
  "services": {},
  "infra": {
    "redis": {"image": "redis", "tag": "7"}
  },
  "env": {"useGlobal": false, "files": []}
}`
	inPath := filepath.Join(dir, "in.json")
	if err := os.WriteFile(inPath, []byte(jsonContent), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	origFrom := migrateYAMLFrom
	origOut := migrateYAMLOutput
	migrateYAMLFrom = inPath
	outPath := filepath.Join(dir, "out.yaml")
	migrateYAMLOutput = outPath
	defer func() {
		migrateYAMLFrom = origFrom
		migrateYAMLOutput = origOut
	}()

	if err := migrateYAMLCmd.RunE(migrateYAMLCmd, []string{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if len(data) == 0 {
		t.Error("output file empty")
	}
}
