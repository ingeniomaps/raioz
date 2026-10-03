package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func loadYAMLString(t *testing.T, body string) (*Deps, error) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "raioz.yaml")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	deps, _, err := LoadDepsFromYAML(path)
	return deps, err
}

func TestResources_DefaultAndOverride(t *testing.T) {
	deps, err := loadYAMLString(t, `version: "1"
project: res
proxy: true
resources:
  memory: 128m
dependencies:
  inherits:
    image: redis:7
  overrides:
    image: postgres:16
    resources:
      memory: 512m
      cpus: 1.5
`)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if got := deps.Infra["inherits"].Inline.Resources; got == nil || got.Memory != "128m" {
		t.Errorf("a dependency without a block takes the root default, got %+v", got)
	}
	got := deps.Infra["overrides"].Inline.Resources
	if got == nil || got.Memory != "512m" || got.CPUs != 1.5 {
		t.Errorf("a dependency's own block replaces the default, got %+v", got)
	}
	if p := deps.ProxyConfig.Resources; p == nil || p.Memory != "128m" {
		t.Errorf("the proxy takes the root default, got %+v", p)
	}
}

func TestResources_NoBlockMeansNoCap(t *testing.T) {
	deps, err := loadYAMLString(t, "project: res\ndependencies:\n  kv:\n    image: redis:7\n")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got := deps.Infra["kv"].Inline.Resources; got != nil {
		t.Errorf("no resources declared anywhere must stay uncapped, got %+v", got)
	}
}

func TestResources_Rejected(t *testing.T) {
	tests := []struct {
		name, body, want string
	}{
		{
			"bad memory on a dependency",
			"project: res\ndependencies:\n  kv:\n    image: redis:7\n    resources:\n      memory: lots\n",
			"dependencies.kv.resources",
		},
		{
			"bad memory on the proxy",
			"project: res\nproxy:\n  resources:\n    memory: 1x\ndependencies:\n  kv:\n    image: redis:7\n",
			"proxy.resources",
		},
		{
			"compose dependency",
			"project: res\ndependencies:\n  kv:\n    compose: ./kv.yml\n    resources:\n      memory: 64m\n",
			"only applies to 'image:'",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := loadYAMLString(t, tt.body)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("want an error mentioning %q, got %v", tt.want, err)
			}
		})
	}
}

func TestResources_Services(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "api"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "api", "Dockerfile"), []byte("FROM scratch\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "raioz.yaml")
	body := `version: "1"
project: res
resources:
  memory: 128m
services:
  api:
    path: ./api
    resources:
      memory: 512m
  worker:
    path: ./api
  host:
    path: ./api
    command: ./run
    resources:
      cpus: 1
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	deps, warnings, err := LoadDepsFromYAML(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if got := deps.Services["api"].Resources; got == nil || got.Memory != "512m" {
		t.Errorf("a service's own block is kept, got %+v", got)
	}
	if got := deps.Services["worker"].Resources; got == nil || got.Memory != "128m" {
		t.Errorf("a service without a block takes the root default, got %+v", got)
	}

	var flagged []string
	for _, w := range warnings {
		if strings.Contains(w, "resources") {
			flagged = append(flagged, w)
		}
	}
	if len(flagged) != 1 || !strings.Contains(flagged[0], "host") {
		t.Errorf("only the host service that declares resources is flagged, got %v", flagged)
	}
}
