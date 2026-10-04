package production

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

const composeFixture = `
services:
  back:
    build: ./back
    ports: ["8080:8080"]
    environment:
      DATABASE_URL: postgres://db
      DEBUG:
    env_file: .env.back
    depends_on: [postgres]
  worker:
    build:
      context: ./worker
      dockerfile: Dockerfile.dev
    environment:
      - QUEUE=jobs
  postgres:
    image: postgres:16
    ports: ["5432:5432"]
    volumes: ["pgdata:/var/lib/postgresql/data"]
    env_file: [.env.pg]
  mail:
    image: axllent/mailpit:v1.20
    depends_on: [postgres]
volumes:
  pgdata: {}
`

func loadFixture(t *testing.T) *ProductionConfig {
	t.Helper()
	path := filepath.Join(t.TempDir(), "docker-compose.yml")
	if err := os.WriteFile(path, []byte(composeFixture), 0o600); err != nil {
		t.Fatal(err)
	}
	prod, err := LoadComposeFile(path)
	if err != nil {
		t.Fatalf("LoadComposeFile: %v", err)
	}
	return prod
}

// `environment:` loads in both of its forms, the mapping (the common one)
// and the list.
func TestLoadComposeFile_EnvironmentForms(t *testing.T) {
	prod := loadFixture(t)

	if got, want := []string(prod.Services["back"].Environment),
		[]string{"DATABASE_URL=postgres://db", "DEBUG"}; !reflect.DeepEqual(got, want) {
		t.Errorf("mapping form = %v, want %v", got, want)
	}
	if got, want := []string(prod.Services["worker"].Environment), []string{"QUEUE=jobs"}; !reflect.DeepEqual(got, want) {
		t.Errorf("list form = %v, want %v", got, want)
	}
	if got := []string(prod.Services["back"].EnvFile); !reflect.DeepEqual(got, []string{".env.back"}) {
		t.Errorf("scalar env_file = %v", got)
	}
	if got := []string(prod.Services["postgres"].EnvFile); !reflect.DeepEqual(got, []string{".env.pg"}) {
		t.Errorf("list env_file = %v", got)
	}
}

// `build:` means code the developer edits (a service); a bare `image:`
// means something they consume (a dependency).
func TestMigrateCompose(t *testing.T) {
	got := MigrateCompose(loadFixture(t))

	if svc, ok := got.Services["back"]; !ok || svc.Path != "./back" ||
		!reflect.DeepEqual(svc.DependsOn, []string{"postgres"}) || !reflect.DeepEqual(svc.EnvFiles, []string{".env.back"}) {
		t.Errorf("back = %+v", svc)
	}
	if svc, ok := got.Services["worker"]; !ok || svc.Path != "./worker" {
		t.Errorf("worker = %+v", svc)
	}
	dep, ok := got.Dependencies["postgres"]
	if !ok || dep.Image != "postgres:16" || len(dep.Volumes) != 1 || len(dep.Ports) != 1 {
		t.Errorf("postgres = %+v", dep)
	}
	if _, ok := got.Dependencies["mail"]; !ok {
		t.Error("mail (image only) should be a dependency")
	}
	if _, isService := got.Services["postgres"]; isService {
		t.Error("an image-only entry became a service")
	}
	if len(got.Warnings) == 0 {
		t.Error("inline environment and dropped depends_on should be reported")
	}
}
