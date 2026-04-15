package docker

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"raioz/internal/config"
)

func TestBuildInlineInfraConfig_Basic(t *testing.T) {
	projectDir := t.TempDir()
	ws := mkWorkspace(t.TempDir())

	deps := &config.Deps{
		Project: config.Project{Name: "proj"},
	}

	infra := config.Infra{
		Image: "postgres",
		Tag:   "16",
		Ports: []string{"5432:5432"},
	}

	got, err := buildInlineInfraConfig(
		"db", infra, deps, ws, projectDir,
		"raioz-net", "proj", false, map[string]string{},
	)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	if got["image"] != "postgres:16" {
		t.Errorf("image = %v, want postgres:16", got["image"])
	}
	if got["container_name"] == nil {
		t.Error("container_name missing")
	}
	ports, ok := got["ports"].([]string)
	if !ok || len(ports) != 1 {
		t.Errorf("ports = %v, want [5432:5432]", got["ports"])
	}
	if _, ok := got["networks"]; !ok {
		t.Error("networks missing")
	}
	// Default postgres env should be applied (no env_file set)
	envVars, ok := got["environment"].(map[string]string)
	if !ok {
		t.Fatalf("environment map missing: got %T", got["environment"])
	}
	if envVars["POSTGRES_PASSWORD"] != "postgres" {
		t.Errorf("missing default POSTGRES_PASSWORD")
	}
	// Default postgres healthcheck
	if _, ok := got["healthcheck"]; !ok {
		t.Error("expected default healthcheck for postgres")
	}
}

func TestBuildInlineInfraConfig_WithIP(t *testing.T) {
	projectDir := t.TempDir()
	ws := mkWorkspace(t.TempDir())
	deps := &config.Deps{Project: config.Project{Name: "proj"}}

	infra := config.Infra{
		Image: "redis",
		IP:    "150.150.0.20",
	}

	got, err := buildInlineInfraConfig(
		"redis", infra, deps, ws, projectDir,
		"raioz-net", "proj", false, map[string]string{},
	)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	nets, ok := got["networks"].(map[string]any)
	if !ok {
		t.Fatalf("networks should be a map with IP config, got %T", got["networks"])
	}
	entry, _ := nets["raioz-net"].(map[string]any)
	if entry["ipv4_address"] != "150.150.0.20" {
		t.Errorf("ipv4_address = %v", entry["ipv4_address"])
	}
}

func TestBuildInlineInfraConfig_WithCustomHealthcheck(t *testing.T) {
	projectDir := t.TempDir()
	ws := mkWorkspace(t.TempDir())
	deps := &config.Deps{Project: config.Project{Name: "proj"}}

	infra := config.Infra{
		Image: "myimage",
		Healthcheck: &config.HealthcheckConfig{
			Test:     []string{"CMD", "check"},
			Interval: "10s",
			Retries:  3,
		},
	}

	got, err := buildInlineInfraConfig(
		"svc", infra, deps, ws, projectDir,
		"raioz-net", "proj", false, map[string]string{},
	)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	hc, ok := got["healthcheck"].(map[string]any)
	if !ok {
		t.Fatalf("healthcheck missing/wrong type: %T", got["healthcheck"])
	}
	if hc["interval"] != "10s" {
		t.Errorf("interval = %v, want 10s", hc["interval"])
	}
}

func TestBuildInlineInfraConfig_WithSeed(t *testing.T) {
	projectDir := t.TempDir()
	// Create a seed file
	seedPath := filepath.Join(projectDir, "seed.sql")
	if err := os.WriteFile(seedPath, []byte("SELECT 1;"), 0644); err != nil {
		t.Fatalf("write seed: %v", err)
	}
	ws := mkWorkspace(t.TempDir())
	deps := &config.Deps{Project: config.Project{Name: "proj"}}

	infra := config.Infra{
		Image: "postgres",
		Seed:  []string{"seed.sql"},
	}

	got, err := buildInlineInfraConfig(
		"db", infra, deps, ws, projectDir,
		"raioz-net", "proj", false, map[string]string{},
	)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	vols, ok := got["volumes"].([]string)
	if !ok || len(vols) == 0 {
		t.Fatalf("expected volumes with seed mount: got %v", got["volumes"])
	}
	// Seed mount should contain /docker-entrypoint-initdb.d
	found := false
	for _, v := range vols {
		if strings.Contains(v, "docker-entrypoint-initdb.d") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("seed volume missing init dir mount: %v", vols)
	}
}

func TestBuildInlineInfraConfig_WithObjectEnv(t *testing.T) {
	projectDir := t.TempDir()
	ws := mkWorkspace(t.TempDir())
	deps := &config.Deps{Project: config.Project{Name: "proj"}}

	infra := config.Infra{
		Image: "custom",
		Env: &config.EnvValue{
			IsObject:  true,
			Variables: map[string]string{"FOO": "bar", "BAZ": "qux"},
		},
	}

	got, err := buildInlineInfraConfig(
		"svc", infra, deps, ws, projectDir,
		"raioz-net", "proj", false, map[string]string{},
	)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	envVars, ok := got["environment"].(map[string]string)
	if !ok {
		t.Fatalf("environment missing: %T", got["environment"])
	}
	if envVars["FOO"] != "bar" {
		t.Errorf("FOO = %q, want bar", envVars["FOO"])
	}
	// No env_file set since it's object variant
	if _, ok := got["env_file"]; ok {
		t.Error("env_file should not be set for object env")
	}
}

// --- resolveInfraEnv ---

func TestResolveInfraEnv_ObjectVars(t *testing.T) {
	ws := mkWorkspace(t.TempDir())
	deps := &config.Deps{Project: config.Project{Name: "proj"}}
	cfg := map[string]any{}

	infra := config.Infra{
		Env: &config.EnvValue{
			IsObject:  true,
			Variables: map[string]string{"A": "1"},
		},
	}

	vars, hasFile, err := resolveInfraEnv(cfg, "svc", infra, deps, ws, t.TempDir())
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if hasFile {
		t.Error("should not have env_file for object vars")
	}
	if vars["A"] != "1" {
		t.Errorf("vars = %v, want A=1", vars)
	}
}

func TestResolveInfraEnv_NilEnv(t *testing.T) {
	ws := mkWorkspace(t.TempDir())
	deps := &config.Deps{Project: config.Project{Name: "proj"}}
	cfg := map[string]any{}

	infra := config.Infra{}

	vars, hasFile, err := resolveInfraEnv(cfg, "svc", infra, deps, ws, t.TempDir())
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if hasFile {
		t.Error("should not have env_file for nil env")
	}
	if len(vars) != 0 {
		t.Errorf("vars = %v, want empty", vars)
	}
}

func TestResolveInfraEnv_DotEnvFile(t *testing.T) {
	projectDir := t.TempDir()
	// Create a local .env
	envFile := filepath.Join(projectDir, ".env")
	if err := os.WriteFile(envFile, []byte("X=y\n"), 0644); err != nil {
		t.Fatalf("write env: %v", err)
	}
	ws := mkWorkspace(t.TempDir())
	deps := &config.Deps{Project: config.Project{Name: "proj"}}
	cfg := map[string]any{}

	infra := config.Infra{
		Env: &config.EnvValue{
			IsObject: false,
			Files:    []string{"."},
		},
	}

	_, hasFile, err := resolveInfraEnv(cfg, "svc", infra, deps, ws, projectDir)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !hasFile {
		t.Error("expected hasFile=true when .env exists")
	}
	if _, ok := cfg["env_file"]; !ok {
		t.Error("env_file should be set in cfg")
	}
}

// --- collectInfraEnvFromFiles ---

func TestCollectInfraEnvFromFiles_DotEnv(t *testing.T) {
	projectDir := t.TempDir()
	envPath := filepath.Join(projectDir, ".env")
	if err := os.WriteFile(envPath, []byte("KEY=val\n"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	target := make(map[string]string)
	ws := mkWorkspace(t.TempDir())
	deps := &config.Deps{Project: config.Project{Name: "proj"}}

	infra := config.Infra{
		Env: &config.EnvValue{
			IsObject: false,
			Files:    []string{"."},
		},
	}

	collectInfraEnvFromFiles(target, "svc", infra, deps, ws, projectDir)

	if target["KEY"] != "val" {
		t.Errorf("target = %v, want KEY=val", target)
	}
}
