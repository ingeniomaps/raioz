package cli

import (
	"reflect"
	"strings"
	"testing"

	"raioz/internal/config"
)

func TestResolveProxyIPForHosts(t *testing.T) {
	t.Run("explicit proxy IP wins", func(t *testing.T) {
		deps := &config.Deps{
			Network:     config.NetworkConfig{Name: "n", Subnet: "172.28.0.0/16", IsObject: true},
			ProxyConfig: &config.ProxyConfig{IP: "172.28.1.1"},
		}
		got, err := resolveProxyIPForHosts(deps)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "172.28.1.1" {
			t.Errorf("got %q, want 172.28.1.1", got)
		}
	})

	t.Run("derived from subnet when proxy.ip absent", func(t *testing.T) {
		deps := &config.Deps{
			Network: config.NetworkConfig{Name: "n", Subnet: "172.28.0.0/16", IsObject: true},
		}
		got, err := resolveProxyIPForHosts(deps)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got == "" {
			t.Error("expected derived IP, got empty")
		}
	})

	t.Run("error when neither subnet nor proxy.ip set", func(t *testing.T) {
		deps := &config.Deps{}
		if _, err := resolveProxyIPForHosts(deps); err == nil {
			t.Error("expected error when no IP source declared")
		}
	})

	t.Run("invalid explicit proxy.ip propagates", func(t *testing.T) {
		deps := &config.Deps{
			Network:     config.NetworkConfig{Name: "n", Subnet: "172.28.0.0/16", IsObject: true},
			ProxyConfig: &config.ProxyConfig{IP: "999.999.999.999"},
		}
		if _, err := resolveProxyIPForHosts(deps); err == nil {
			t.Error("expected validation error for malformed IP")
		}
	})
}

func TestProxiedHostnamesFromConfig(t *testing.T) {
	t.Run("services and HTTP deps included, sorted", func(t *testing.T) {
		deps := &config.Deps{
			Services: map[string]config.Service{
				"web": {},
				"api": {Hostname: "api-v2"},
			},
			Infra: map[string]config.InfraEntry{
				"adminer":  {Inline: &config.Infra{Image: "adminer"}},
				"postgres": {Inline: &config.Infra{Image: "postgres:16"}},
			},
		}
		got := proxiedHostnamesFromConfig(deps)
		want := []string{"adminer.localhost", "api-v2.localhost", "web.localhost"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v (postgres should be filtered as non-HTTP)", got, want)
		}
	})

	t.Run("custom domain used for suffix", func(t *testing.T) {
		deps := &config.Deps{
			Services:    map[string]config.Service{"api": {}},
			ProxyConfig: &config.ProxyConfig{Domain: "acme.dev"},
		}
		got := proxiedHostnamesFromConfig(deps)
		if len(got) != 1 || !strings.HasSuffix(got[0], ".acme.dev") {
			t.Errorf("got %v, want suffix .acme.dev", got)
		}
	})

	t.Run("non-HTTP dep with routing override is included", func(t *testing.T) {
		deps := &config.Deps{
			Infra: map[string]config.InfraEntry{
				"postgres": {Inline: &config.Infra{
					Image:   "postgres:16",
					Routing: &config.RoutingConfig{},
				}},
			},
		}
		got := proxiedHostnamesFromConfig(deps)
		if len(got) != 1 || got[0] != "postgres.localhost" {
			t.Errorf("got %v, want [postgres.localhost]", got)
		}
	})

	t.Run("path-based infra entries skipped", func(t *testing.T) {
		deps := &config.Deps{
			Infra: map[string]config.InfraEntry{
				"adminer": {Path: "deps/adminer.yml"}, // no Inline
			},
		}
		got := proxiedHostnamesFromConfig(deps)
		if len(got) != 0 {
			t.Errorf("got %v, want empty (path entries have no inline image)", got)
		}
	})
}

func TestWorkspaceLabel(t *testing.T) {
	t.Run("workspace wins when set", func(t *testing.T) {
		deps := &config.Deps{
			Workspace: "acme",
			Project:   config.Project{Name: "store"},
		}
		if got := workspaceLabel(deps); got != "acme" {
			t.Errorf("got %q, want acme", got)
		}
	})

	t.Run("falls back to project name", func(t *testing.T) {
		deps := &config.Deps{
			Project: config.Project{Name: "store"},
		}
		if got := workspaceLabel(deps); got != "store" {
			t.Errorf("got %q, want store", got)
		}
	})
}
