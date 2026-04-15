package config

import (
	"testing"

	"gopkg.in/yaml.v3"
)

// TestYAMLNetwork_StringShorthand: `network: my-net` parses as name only.
func TestYAMLNetwork_StringShorthand(t *testing.T) {
	var cfg RaiozConfig
	input := "project: p\nnetwork: my-net\n"
	if err := yaml.Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if cfg.Network == nil || cfg.Network.Name != "my-net" {
		t.Errorf("expected name=my-net, got %+v", cfg.Network)
	}
	if cfg.Network.Subnet != "" {
		t.Errorf("subnet should be empty, got %q", cfg.Network.Subnet)
	}
}

// TestYAMLNetwork_ObjectForm: both name and subnet set.
func TestYAMLNetwork_ObjectForm(t *testing.T) {
	var cfg RaiozConfig
	input := "project: p\nnetwork:\n  name: acme-net\n  subnet: 172.28.0.0/16\n"
	if err := yaml.Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if cfg.Network.Name != "acme-net" || cfg.Network.Subnet != "172.28.0.0/16" {
		t.Errorf("expected acme-net + subnet, got %+v", cfg.Network)
	}
}

// TestYAMLNetwork_SubnetOnly: user pins subnet, leaves name to raioz.
func TestYAMLNetwork_SubnetOnly(t *testing.T) {
	var cfg RaiozConfig
	input := "project: p\nnetwork:\n  subnet: 150.150.0.0/16\n"
	if err := yaml.Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if cfg.Network.Name != "" || cfg.Network.Subnet != "150.150.0.0/16" {
		t.Errorf("expected subnet-only, got %+v", cfg.Network)
	}
}

// TestYAMLToDeps_NetworkFallback: no network: block → legacy default applies.
func TestYAMLToDeps_NetworkFallback_NoWorkspace(t *testing.T) {
	cfg := &RaiozConfig{Project: "solo"}
	deps, err := YAMLToDeps(cfg)
	if err != nil {
		t.Fatalf("%v", err)
	}
	if deps.Network.Name != "solo-net" {
		t.Errorf("expected solo-net, got %q", deps.Network.Name)
	}
	if deps.Network.Subnet != "" {
		t.Errorf("subnet must stay empty without explicit subnet, got %q", deps.Network.Subnet)
	}
}

func TestYAMLToDeps_NetworkFallback_WithWorkspace(t *testing.T) {
	cfg := &RaiozConfig{Workspace: "acme", Project: "api"}
	deps, err := YAMLToDeps(cfg)
	if err != nil {
		t.Fatalf("%v", err)
	}
	if deps.Network.Name != "acme-net" {
		t.Errorf("expected acme-net, got %q", deps.Network.Name)
	}
}

// TestYAMLToDeps_NetworkUserSupplied_NameOnly: user override beats default.
func TestYAMLToDeps_NetworkUserSupplied_NameOnly(t *testing.T) {
	cfg := &RaiozConfig{
		Workspace: "acme",
		Project:   "api",
		Network:   &YAMLNetwork{Name: "shared-external"},
	}
	deps, err := YAMLToDeps(cfg)
	if err != nil {
		t.Fatalf("%v", err)
	}
	if deps.Network.Name != "shared-external" {
		t.Errorf("expected shared-external, got %q", deps.Network.Name)
	}
	if deps.Network.IsObject {
		t.Errorf("IsObject should be false when only name is set")
	}
}

// TestYAMLToDeps_NetworkUserSupplied_SubnetOnly: user pins subnet, name derived.
func TestYAMLToDeps_NetworkUserSupplied_SubnetOnly(t *testing.T) {
	cfg := &RaiozConfig{
		Workspace: "acme",
		Project:   "api",
		Network:   &YAMLNetwork{Subnet: "172.28.0.0/16"},
	}
	deps, err := YAMLToDeps(cfg)
	if err != nil {
		t.Fatalf("%v", err)
	}
	if deps.Network.Name != "acme-net" {
		t.Errorf("name should fall back to workspace-derived, got %q", deps.Network.Name)
	}
	if deps.Network.Subnet != "172.28.0.0/16" {
		t.Errorf("subnet lost, got %q", deps.Network.Subnet)
	}
	if !deps.Network.IsObject {
		t.Errorf("IsObject must be true when subnet is present (drives ipam emission)")
	}
}

// TestYAMLToDeps_NetworkUserSupplied_Both: name + subnet both honored.
func TestYAMLToDeps_NetworkUserSupplied_Both(t *testing.T) {
	cfg := &RaiozConfig{
		Project: "api",
		Network: &YAMLNetwork{Name: "deterministic-net", Subnet: "10.20.0.0/16"},
	}
	deps, err := YAMLToDeps(cfg)
	if err != nil {
		t.Fatalf("%v", err)
	}
	if deps.Network.Name != "deterministic-net" ||
		deps.Network.Subnet != "10.20.0.0/16" ||
		!deps.Network.IsObject {
		t.Errorf("round-trip lost fields: %+v", deps.Network)
	}
}

// TestYAMLToDeps_ServiceProxyOverride: user-declared proxy block bridges to
// Service.ProxyOverride so the orchestrator can bypass detection.
func TestYAMLToDeps_ServiceProxyOverride(t *testing.T) {
	cfg := &RaiozConfig{
		Project: "hypixo",
		Services: map[string]YAMLService{
			"keycloak": {
				Path:    "./kc",
				Command: "make start",
				Proxy: &YAMLServiceProxy{
					Target: "hypixo-keycloak",
					Port:   8080,
				},
			},
		},
	}
	deps, err := YAMLToDeps(cfg)
	if err != nil {
		t.Fatalf("%v", err)
	}
	svc := deps.Services["keycloak"]
	if svc.ProxyOverride == nil {
		t.Fatal("ProxyOverride must be populated from yaml `proxy:`")
	}
	if svc.ProxyOverride.Target != "hypixo-keycloak" || svc.ProxyOverride.Port != 8080 {
		t.Errorf("bridged override wrong: %+v", svc.ProxyOverride)
	}
}

func TestYAMLToDeps_ServiceProxyOverride_Absent(t *testing.T) {
	cfg := &RaiozConfig{
		Project: "p",
		Services: map[string]YAMLService{
			"api": {Path: "./a"},
		},
	}
	deps, _ := YAMLToDeps(cfg)
	if deps.Services["api"].ProxyOverride != nil {
		t.Error("override must stay nil when yaml omits the proxy block")
	}
}

// TestYAMLToDeps_DependencyProxyOverride: `dependencies.<n>.proxy:` bridges
// into Infra.ProxyOverride so the orchestrator can steer Caddy at a
// non-default container/port. Regression guard for v0.1.0 where the field
// was silently dropped.
func TestYAMLToDeps_DependencyProxyOverride(t *testing.T) {
	cfg := &RaiozConfig{
		Project: "hypixo",
		Deps: map[string]YAMLDependency{
			"redisinsight": {
				Image: "redis/redisinsight:latest",
				Proxy: &YAMLServiceProxy{
					Target: "hypixo-redisinsight",
					Port:   5540,
				},
			},
		},
	}
	deps, err := YAMLToDeps(cfg)
	if err != nil {
		t.Fatalf("%v", err)
	}
	entry := deps.Infra["redisinsight"]
	if entry.Inline == nil || entry.Inline.ProxyOverride == nil {
		t.Fatal("Infra.ProxyOverride must be populated from yaml dependency `proxy:`")
	}
	if entry.Inline.ProxyOverride.Target != "hypixo-redisinsight" ||
		entry.Inline.ProxyOverride.Port != 5540 {
		t.Errorf("bridged dep override wrong: %+v", entry.Inline.ProxyOverride)
	}
}

func TestYAMLToDeps_DependencyProxyOverride_Absent(t *testing.T) {
	cfg := &RaiozConfig{
		Project: "p",
		Deps: map[string]YAMLDependency{
			"postgres": {Image: "postgres:16"},
		},
	}
	deps, _ := YAMLToDeps(cfg)
	if deps.Infra["postgres"].Inline.ProxyOverride != nil {
		t.Error("override must stay nil when dep yaml omits the proxy block")
	}
}
