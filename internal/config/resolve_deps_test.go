package config

import (
	"slices"
	"sort"
	"testing"
)

func TestResolveDependencies_SingleService(t *testing.T) {
	deps := &Deps{
		Services: map[string]Service{
			"api": {},
		},
		Infra: map[string]InfraEntry{},
	}

	services, infra := ResolveDependencies(deps, []string{"api"})
	if len(services) != 1 || services[0] != "api" {
		t.Errorf("expected [api], got %v", services)
	}
	if len(infra) != 0 {
		t.Errorf("expected no infra, got %v", infra)
	}
}

func TestResolveDependencies_ServiceWithInfraDep(t *testing.T) {
	deps := &Deps{
		Services: map[string]Service{
			"api": {
				Docker: &DockerConfig{
					DependsOn: []string{"database"},
				},
			},
		},
		Infra: map[string]InfraEntry{
			"database": {Inline: &Infra{Image: "postgres"}},
		},
	}

	services, infra := ResolveDependencies(deps, []string{"api"})
	if len(services) != 1 || services[0] != "api" {
		t.Errorf("expected [api], got %v", services)
	}
	if len(infra) != 1 || infra[0] != "database" {
		t.Errorf("expected [database], got %v", infra)
	}
}

func TestResolveDependencies_TransitiveDeps(t *testing.T) {
	deps := &Deps{
		Services: map[string]Service{
			"frontend": {
				Docker: &DockerConfig{
					DependsOn: []string{"api"},
				},
			},
			"api": {
				Docker: &DockerConfig{
					DependsOn: []string{"database", "redis"},
				},
			},
		},
		Infra: map[string]InfraEntry{
			"database": {Inline: &Infra{Image: "postgres"}},
			"redis":    {Inline: &Infra{Image: "redis"}},
		},
	}

	services, infra := ResolveDependencies(deps, []string{"frontend"})

	sort.Strings(services)
	sort.Strings(infra)

	if !slices.Equal(services, []string{"api", "frontend"}) {
		t.Errorf("expected [api, frontend], got %v", services)
	}
	if !slices.Equal(infra, []string{"database", "redis"}) {
		t.Errorf("expected [database, redis], got %v", infra)
	}
}

func TestResolveDependencies_InfraOnly(t *testing.T) {
	deps := &Deps{
		Services: map[string]Service{},
		Infra: map[string]InfraEntry{
			"postgres": {Inline: &Infra{Image: "postgres"}},
			"redis":    {Inline: &Infra{Image: "redis"}},
		},
	}

	services, infra := ResolveDependencies(deps, []string{"postgres"})
	if len(services) != 0 {
		t.Errorf("expected no services, got %v", services)
	}
	if len(infra) != 1 || infra[0] != "postgres" {
		t.Errorf("expected [postgres], got %v", infra)
	}
}

func TestFilterByServices(t *testing.T) {
	deps := &Deps{
		Project: Project{Name: "test"},
		Services: map[string]Service{
			"api":      {},
			"frontend": {},
			"worker":   {},
		},
		Infra: map[string]InfraEntry{
			"postgres": {Inline: &Infra{Image: "postgres"}},
			"redis":    {Inline: &Infra{Image: "redis"}},
			"mongo":    {Inline: &Infra{Image: "mongo"}},
		},
	}

	filtered := FilterByServices(deps, []string{"api"}, []string{"postgres"})

	if len(filtered.Services) != 1 {
		t.Errorf("expected 1 service, got %d", len(filtered.Services))
	}
	if _, ok := filtered.Services["api"]; !ok {
		t.Error("expected api in filtered services")
	}
	if len(filtered.Infra) != 1 {
		t.Errorf("expected 1 infra, got %d", len(filtered.Infra))
	}
	if filtered.Project.Name != "test" {
		t.Error("project should be preserved")
	}
}
