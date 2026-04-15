package config

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestYAMLWatch_UnmarshalBool(t *testing.T) {
	tests := []struct {
		yaml    string
		enabled bool
		mode    string
	}{
		{"true", true, ""},
		{"false", false, ""},
	}
	for _, tt := range tests {
		t.Run(tt.yaml, func(t *testing.T) {
			var w YAMLWatch
			if err := yaml.Unmarshal([]byte(tt.yaml), &w); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if w.Enabled != tt.enabled {
				t.Errorf("Enabled = %v, want %v", w.Enabled, tt.enabled)
			}
			if w.Mode != tt.mode {
				t.Errorf("Mode = %q, want %q", w.Mode, tt.mode)
			}
		})
	}
}

func TestYAMLWatch_UnmarshalString(t *testing.T) {
	var w YAMLWatch
	if err := yaml.Unmarshal([]byte(`"native"`), &w); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !w.Enabled {
		t.Error("expected Enabled=true for string mode")
	}
	if w.Mode != "native" {
		t.Errorf("Mode = %q, want %q", w.Mode, "native")
	}
}

func TestYAMLStringSlice_Single(t *testing.T) {
	var s YAMLStringSlice
	if err := yaml.Unmarshal([]byte(`"single"`), &s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(s) != 1 || s[0] != "single" {
		t.Errorf("got %v, want [single]", s)
	}
}

func TestYAMLStringSlice_List(t *testing.T) {
	var s YAMLStringSlice
	if err := yaml.Unmarshal([]byte("[a, b, c]"), &s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(s) != 3 {
		t.Errorf("len = %d, want 3", len(s))
	}
}

func TestYAMLStringOrSlice_Single(t *testing.T) {
	var s YAMLStringOrSlice
	if err := yaml.Unmarshal([]byte(`"echo hello"`), &s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(s) != 1 || s[0] != "echo hello" {
		t.Errorf("got %v", s)
	}
}

func TestYAMLStringOrSlice_List(t *testing.T) {
	var s YAMLStringOrSlice
	if err := yaml.Unmarshal([]byte("[cmd1, cmd2]"), &s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(s) != 2 {
		t.Errorf("len = %d, want 2", len(s))
	}
}

func TestYAMLIntSlice_Single(t *testing.T) {
	var s YAMLIntSlice
	if err := yaml.Unmarshal([]byte("5432"), &s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(s) != 1 || s[0] != 5432 {
		t.Errorf("got %v, want [5432]", s)
	}
}

func TestYAMLIntSlice_List(t *testing.T) {
	var s YAMLIntSlice
	if err := yaml.Unmarshal([]byte("[5432, 9090]"), &s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(s) != 2 || s[0] != 5432 || s[1] != 9090 {
		t.Errorf("got %v", s)
	}
}

func TestYAMLPublish_Bool(t *testing.T) {
	tests := []struct {
		yaml string
		auto bool
	}{
		{"true", true},
		{"false", false},
	}
	for _, tt := range tests {
		t.Run(tt.yaml, func(t *testing.T) {
			var p YAMLPublish
			if err := yaml.Unmarshal([]byte(tt.yaml), &p); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if !p.Set {
				t.Error("Set should be true")
			}
			if p.Auto != tt.auto {
				t.Errorf("Auto = %v, want %v", p.Auto, tt.auto)
			}
		})
	}
}

func TestYAMLPublish_SingleInt(t *testing.T) {
	var p YAMLPublish
	if err := yaml.Unmarshal([]byte("5432"), &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !p.Set {
		t.Error("Set should be true")
	}
	if len(p.Ports) != 1 || p.Ports[0] != 5432 {
		t.Errorf("Ports = %v, want [5432]", p.Ports)
	}
}

func TestYAMLPublish_IntList(t *testing.T) {
	var p YAMLPublish
	if err := yaml.Unmarshal([]byte("[5432, 9090]"), &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !p.Set {
		t.Error("Set should be true")
	}
	if len(p.Ports) != 2 {
		t.Errorf("Ports len = %d, want 2", len(p.Ports))
	}
}

func TestProxyConfig_UnmarshalBool(t *testing.T) {
	var p ProxyConfig
	if err := yaml.Unmarshal([]byte("true"), &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !p.Enabled {
		t.Error("expected Enabled=true")
	}
}

func TestProxyConfig_UnmarshalObject(t *testing.T) {
	data := `
mode: path
domain: dev.acme.com
tls: letsencrypt
`
	var p ProxyConfig
	if err := yaml.Unmarshal([]byte(data), &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !p.Enabled {
		t.Error("expected Enabled=true for object")
	}
	if p.Mode != "path" {
		t.Errorf("Mode = %q, want %q", p.Mode, "path")
	}
	if p.Domain != "dev.acme.com" {
		t.Errorf("Domain = %q", p.Domain)
	}
	if p.TLS != "letsencrypt" {
		t.Errorf("TLS = %q", p.TLS)
	}
}

func TestRaiozConfig_FullParse(t *testing.T) {
	data := `
workspace: acme
project: api
proxy: true
pre: ./setup.sh
post:
  - rm -f .env.tmp
  - echo done
services:
  api:
    path: ./api
    dependsOn: postgres
    watch: true
    health: /health
    port: 3000
  web:
    path: ./web
    command: make dev
    stop: make stop
    compose: docker-compose.yml
dependencies:
  postgres:
    image: postgres:16
    ports: ["5432"]
    expose: 5432
    publish: true
  redis:
    image: redis:7
    publish: 6379
`
	var cfg RaiozConfig
	if err := yaml.Unmarshal([]byte(data), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if cfg.Workspace != "acme" {
		t.Errorf("Workspace = %q", cfg.Workspace)
	}
	if cfg.Project != "api" {
		t.Errorf("Project = %q", cfg.Project)
	}
	if !cfg.Proxy.Enabled {
		t.Error("Proxy should be enabled")
	}
	if len(cfg.Pre) != 1 || cfg.Pre[0] != "./setup.sh" {
		t.Errorf("Pre = %v", cfg.Pre)
	}
	if len(cfg.Post) != 2 {
		t.Errorf("Post len = %d, want 2", len(cfg.Post))
	}

	api := cfg.Services["api"]
	if api.Path != "./api" {
		t.Errorf("api.Path = %q", api.Path)
	}
	if len(api.DependsOn) != 1 || api.DependsOn[0] != "postgres" {
		t.Errorf("api.DependsOn = %v", api.DependsOn)
	}
	if !api.Watch.Enabled {
		t.Error("api.Watch should be enabled")
	}
	if api.Port != 3000 {
		t.Errorf("api.Port = %d", api.Port)
	}

	web := cfg.Services["web"]
	if web.Command != "make dev" {
		t.Errorf("web.Command = %q", web.Command)
	}
	if web.Stop != "make stop" {
		t.Errorf("web.Stop = %q", web.Stop)
	}
	if len(web.Compose) != 1 || web.Compose[0] != "docker-compose.yml" {
		t.Errorf("web.Compose = %v", web.Compose)
	}

	pg := cfg.Deps["postgres"]
	if pg.Image != "postgres:16" {
		t.Errorf("pg.Image = %q", pg.Image)
	}
	if len(pg.Expose) != 1 || pg.Expose[0] != 5432 {
		t.Errorf("pg.Expose = %v", pg.Expose)
	}
	if !pg.Publish.Auto {
		t.Error("pg.Publish.Auto should be true")
	}

	redis := cfg.Deps["redis"]
	if len(redis.Publish.Ports) != 1 || redis.Publish.Ports[0] != 6379 {
		t.Errorf("redis.Publish.Ports = %v", redis.Publish.Ports)
	}
}
