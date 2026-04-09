package detect

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInferDepsFromEnv_Postgres(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".env", "DATABASE_URL=postgres://user:pass@localhost:5432/mydb\n")

	deps, _ := InferDepsFromEnv(dir)

	if len(deps) != 1 {
		t.Fatalf("expected 1 dep, got %d", len(deps))
	}
	if deps[0].Name != "postgres" {
		t.Errorf("expected postgres, got %s", deps[0].Name)
	}
	if deps[0].Image != "postgres:16" {
		t.Errorf("expected postgres:16, got %s", deps[0].Image)
	}
	if deps[0].Port != "5432" {
		t.Errorf("expected 5432, got %s", deps[0].Port)
	}
}

func TestInferDepsFromEnv_Multiple(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".env", `
DATABASE_URL=postgres://localhost:5432/db
REDIS_URL=redis://localhost:6379
RABBITMQ_URL=amqp://localhost:5672
`)

	deps, _ := InferDepsFromEnv(dir)

	names := make(map[string]bool)
	for _, dep := range deps {
		names[dep.Name] = true
	}

	if !names["postgres"] {
		t.Error("expected postgres")
	}
	if !names["redis"] {
		t.Error("expected redis")
	}
	if !names["rabbitmq"] {
		t.Error("expected rabbitmq")
	}
}

func TestInferDepsFromEnv_SubdirLinks(t *testing.T) {
	dir := t.TempDir()
	apiDir := filepath.Join(dir, "api")
	os.MkdirAll(apiDir, 0755)

	writeFile(t, apiDir, ".env", "DATABASE_URL=postgres://localhost:5432/api\nREDIS_HOST=localhost\n")

	deps, links := InferDepsFromEnv(dir)

	if len(deps) < 2 {
		t.Fatalf("expected at least 2 deps, got %d", len(deps))
	}

	// Should have links from api to postgres and redis
	apiLinks := 0
	for _, link := range links {
		if link.From == "api" {
			apiLinks++
		}
	}
	if apiLinks < 2 {
		t.Errorf("expected at least 2 links from api, got %d", apiLinks)
	}
}

func TestInferDepsFromEnv_EnvVarNameMatching(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".env", `
POSTGRES_HOST=localhost
POSTGRES_PORT=5432
POSTGRES_USER=admin
REDIS_HOST=cache.local
`)

	deps, _ := InferDepsFromEnv(dir)

	names := make(map[string]bool)
	for _, dep := range deps {
		names[dep.Name] = true
	}

	if !names["postgres"] {
		t.Error("expected postgres from POSTGRES_HOST")
	}
	if !names["redis"] {
		t.Error("expected redis from REDIS_HOST")
	}
}

func TestInferDepsFromEnv_IgnoresComments(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".env", `
# This is a comment
# DATABASE_URL=postgres://localhost/old
APP_NAME=myapp
`)

	deps, _ := InferDepsFromEnv(dir)

	if len(deps) != 0 {
		t.Errorf("expected 0 deps (comments and non-matching), got %d", len(deps))
	}
}

func TestInferDepsFromEnv_NoDuplicates(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".env", `
DATABASE_URL=postgres://localhost:5432/db
POSTGRES_HOST=localhost
PG_PASSWORD=secret
`)

	deps, _ := InferDepsFromEnv(dir)

	pgCount := 0
	for _, dep := range deps {
		if dep.Name == "postgres" {
			pgCount++
		}
	}
	if pgCount != 1 {
		t.Errorf("expected 1 postgres dep (no duplicates), got %d", pgCount)
	}
}

func TestInferDepsFromEnv_MongoAndMySQL(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".env", `
MONGO_URL=mongodb://localhost:27017/mydb
MYSQL_HOST=localhost
`)

	deps, _ := InferDepsFromEnv(dir)

	names := make(map[string]bool)
	for _, dep := range deps {
		names[dep.Name] = true
	}

	if !names["mongodb"] {
		t.Error("expected mongodb")
	}
	if !names["mysql"] {
		t.Error("expected mysql")
	}
}

func TestIsInfraImage(t *testing.T) {
	tests := []struct {
		image string
		want  bool
	}{
		{"postgres:16", true},
		{"redis:7", true},
		{"mysql:8", true},
		{"mongo:7", true},
		{"rabbitmq:3-management", true},
		{"node:18", false},
		{"myapp:latest", false},
		{"acme/auth-service:v1", false},
		{"bitnami/redis:latest", true},
	}

	for _, tt := range tests {
		if got := isInfraImage(tt.image); got != tt.want {
			t.Errorf("isInfraImage(%q) = %v, want %v", tt.image, got, tt.want)
		}
	}
}

func TestIsInfraName(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"postgres", true},
		{"db", true},
		{"redis", true},
		{"cache", true},
		{"api", false},
		{"frontend", false},
		{"my-redis", true},
		{"database", true},
	}

	for _, tt := range tests {
		if got := isInfraName(tt.name); got != tt.want {
			t.Errorf("isInfraName(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}
