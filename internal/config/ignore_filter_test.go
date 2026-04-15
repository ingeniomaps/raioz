package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestFilterIgnoredServices_NoIgnored(t *testing.T) {
	home := t.TempDir()
	t.Setenv("RAIOZ_HOME", home)
	// No ignore.json — returns deps unchanged

	deps := &Deps{
		Services: map[string]Service{
			"api":  {},
			"web":  {},
		},
		Infra: map[string]InfraEntry{},
	}

	filtered, ignored, err := FilterIgnoredServices(deps)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(ignored) != 0 {
		t.Errorf("expected 0 ignored, got %d", len(ignored))
	}
	if len(filtered.Services) != 2 {
		t.Errorf("expected 2 services, got %d", len(filtered.Services))
	}
}

func TestFilterIgnoredServices_WithIgnored(t *testing.T) {
	home := t.TempDir()
	t.Setenv("RAIOZ_HOME", home)

	// Create ignore.json with "web" ignored
	ignoreConfig := map[string]interface{}{
		"services": []string{"web"},
	}
	data, _ := json.Marshal(ignoreConfig)
	os.WriteFile(filepath.Join(home, "ignore.json"), data, 0o644)

	deps := &Deps{
		Services: map[string]Service{
			"api":  {},
			"web":  {},
			"auth": {},
		},
		Infra: map[string]InfraEntry{},
	}

	filtered, ignored, err := FilterIgnoredServices(deps)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(ignored) != 1 || ignored[0] != "web" {
		t.Errorf("expected [web] ignored, got %v", ignored)
	}
	if _, ok := filtered.Services["web"]; ok {
		t.Error("web should be filtered out")
	}
	if _, ok := filtered.Services["api"]; !ok {
		t.Error("api should remain")
	}
	if _, ok := filtered.Services["auth"]; !ok {
		t.Error("auth should remain")
	}
}

func TestCheckIgnoredDependencies_NoIgnored(t *testing.T) {
	deps := &Deps{
		Services: map[string]Service{
			"api": {DependsOn: []string{"postgres"}},
		},
	}
	result := CheckIgnoredDependencies(deps, nil)
	if result != nil {
		t.Errorf("expected nil for empty ignored list, got %v", result)
	}
}

func TestCheckIgnoredDependencies_FindsIgnoredDeps(t *testing.T) {
	deps := &Deps{
		Services: map[string]Service{
			"api":    {DependsOn: []string{"postgres", "redis"}},
			"worker": {DependsOn: []string{"redis"}},
			"redis":  {}, // this one is ignored
		},
	}

	result := CheckIgnoredDependencies(deps, []string{"redis"})
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if len(result["api"]) != 1 || result["api"][0] != "redis" {
		t.Errorf("api deps = %v, want [redis]", result["api"])
	}
	if len(result["worker"]) != 1 || result["worker"][0] != "redis" {
		t.Errorf("worker deps = %v, want [redis]", result["worker"])
	}
	// redis itself should not appear since it's in the ignored set
	if _, ok := result["redis"]; ok {
		t.Error("ignored service itself should not appear as having ignored deps")
	}
}

func TestCheckIgnoredDependencies_DockerDependsOn(t *testing.T) {
	deps := &Deps{
		Services: map[string]Service{
			"api": {
				Docker: &DockerConfig{DependsOn: []string{"cache"}},
			},
		},
	}

	result := CheckIgnoredDependencies(deps, []string{"cache"})
	if len(result["api"]) != 1 || result["api"][0] != "cache" {
		t.Errorf("expected cache in ignored deps for api, got %v", result["api"])
	}
}

func TestCheckIgnoredDependencies_NoOverlap(t *testing.T) {
	deps := &Deps{
		Services: map[string]Service{
			"api": {DependsOn: []string{"postgres"}},
		},
	}

	result := CheckIgnoredDependencies(deps, []string{"redis"})
	if len(result) != 0 {
		t.Errorf("expected empty map, got %v", result)
	}
}
