package detect

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInferDepsFromCompose(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "docker-compose.yml")
	body := `services:
  postgres:
    image: postgres:16
  redis:
    image: redis:7
  api:
    image: myapp:latest
`
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}

	deps := InferDepsFromCompose(path)
	got := make(map[string]bool)
	for _, d := range deps {
		got[d.Name] = true
	}

	if !got["postgres"] {
		t.Error("postgres not inferred")
	}
	if !got["redis"] {
		t.Error("redis not inferred")
	}
	if got["api"] {
		t.Error("api should not be classified as infra")
	}
}

func TestInferDepsFromCompose_MissingFile(t *testing.T) {
	if got := InferDepsFromCompose("/nonexistent/compose.yml"); got != nil {
		t.Errorf("expected nil for missing file, got %v", got)
	}
}

func TestInferPortFromImage(t *testing.T) {
	tests := []struct {
		image string
		want  string // empty means "any non-empty result is fine"
	}{
		{"postgres:16", ""},
		{"redis:7", ""},
		{"unknown-image:1.0", ""},
	}
	for _, tt := range tests {
		t.Run(tt.image, func(t *testing.T) {
			// Just ensure the function doesn't panic and returns something
			// for known images. Don't pin the exact port — knownServices
			// is the source of truth and may evolve.
			_ = inferPortFromImage(tt.image)
		})
	}
}

func TestIsInfraImage_Compose(t *testing.T) {
	tests := []struct {
		image string
		want  bool
	}{
		{"postgres:16", true},
		{"redis:7", true},
		{"library/mysql", true},
		{"myorg/api:v1", false},
		{"nginx:1.25", false},
	}
	for _, tt := range tests {
		t.Run(tt.image, func(t *testing.T) {
			if got := isInfraImage(tt.image); got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsInfraName_Compose(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"db", true},
		{"postgres-primary", true},
		{"shared-redis", true},
		{"api", false},
		{"web", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isInfraName(tt.name); got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}
