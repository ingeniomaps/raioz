package production

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

func TestLoadComposeFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "docker-compose.yml")
	yaml := `services:
  api:
    image: node:18
    ports: ["3000:3000"]
  db:
    image: postgres:16
`
	if err := os.WriteFile(path, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadComposeFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Services) != 2 {
		t.Errorf("expected 2 services, got %d", len(cfg.Services))
	}
}

func TestLoadComposeFile_MissingFile(t *testing.T) {
	if _, err := LoadComposeFile("/nonexistent.yml"); err == nil {
		t.Error("expected error for missing file")
	}
}

func TestLoadComposeFile_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yml")
	if err := os.WriteFile(path, []byte("services:\n  - not-a-map"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadComposeFile(path); err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestNormalizePorts(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{"nil → empty", nil, []string{}},
		{"simple host:container", []string{"3000:3000"}, []string{"3000:3000"}},
		{"with protocol stripped", []string{"3000:3000/tcp"}, []string{"3000:3000"}},
		{"bind address dropped", []string{"127.0.0.1:3000:3000"}, []string{"3000:3000"}},
		{"single port preserved", []string{"3000"}, []string{"3000"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizePorts(tt.in)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseDependsOn(t *testing.T) {
	tests := []struct {
		name string
		in   interface{}
		want []string
	}{
		{"nil", nil, []string{}},
		{"[]string", []string{"db", "redis"}, []string{"db", "redis"}},
		{"[]interface{} of strings", []interface{}{"db", "redis"}, []string{"db", "redis"}},
		{
			"map with conditions",
			map[string]interface{}{"db": map[string]interface{}{"condition": "service_healthy"}},
			[]string{"db"},
		},
		{
			"[]interface{} of maps",
			[]interface{}{map[string]interface{}{"db": map[string]interface{}{}}},
			[]string{"db"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseDependsOn(tt.in)
			sort.Strings(got)
			sort.Strings(tt.want)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtractImageAndTag(t *testing.T) {
	tests := []struct {
		in        string
		wantImage string
		wantTag   string
	}{
		{"postgres:16", "postgres", "16"},
		{"postgres", "postgres", "latest"},
		{"", "", ""},
		{"registry.example.com:5000/app:v1.2", "registry.example.com:5000/app", "v1.2"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			image, tag := ExtractImageAndTag(tt.in)
			if image != tt.wantImage || tag != tt.wantTag {
				t.Errorf("got (%q, %q), want (%q, %q)", image, tag, tt.wantImage, tt.wantTag)
			}
		})
	}
}

func TestIsServiceNameAndGetServiceNames(t *testing.T) {
	cfg := &ProductionConfig{
		Services: map[string]ProductionService{
			"api": {},
			"db":  {},
		},
	}
	if !cfg.IsServiceName("api") {
		t.Error("api should be a known service")
	}
	if cfg.IsServiceName("ghost") {
		t.Error("ghost should not be a known service")
	}

	names := cfg.GetServiceNames()
	sort.Strings(names)
	want := []string{"api", "db"}
	if !reflect.DeepEqual(names, want) {
		t.Errorf("got %v, want %v", names, want)
	}
}

func TestResolveAbsolutePath(t *testing.T) {
	tests := []struct {
		name        string
		composePath string
		target      string
		want        string
	}{
		{"absolute target unchanged", "/tmp/docker-compose.yml", "/etc/passwd", "/etc/passwd"},
		{"relative resolved against compose dir", "/srv/app/compose.yml", "data", "/srv/app/data"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveAbsolutePath(tt.composePath, tt.target)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
