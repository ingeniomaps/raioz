package config

import (
	"strings"
	"testing"
)

// TestYAMLToDeps_ServiceDepCollision guards BUG-9: a name that appears on both
// `services:` and `dependencies:` must fail-fast at load time so the user sees
// a descriptive error instead of undefined docker-compose-project behavior.
func TestYAMLToDeps_ServiceDepCollision(t *testing.T) {
	cfg := &RaiozConfig{
		Project: "p",
		Services: map[string]YAMLService{
			"postgres": {Path: "./db"},
		},
		Deps: map[string]YAMLDependency{
			"postgres": {Image: "postgres:16"},
		},
	}

	_, err := YAMLToDeps(cfg)
	if err == nil {
		t.Fatal("expected error on service/dep name collision, got nil")
	}
	if !strings.Contains(err.Error(), "postgres") {
		t.Errorf("error should mention offending name, got: %v", err)
	}
	if !strings.Contains(err.Error(), "collision") {
		t.Errorf("error should be explicit about the collision, got: %v", err)
	}
}

func TestYAMLToDeps_NoCollisionAllowed(t *testing.T) {
	cfg := &RaiozConfig{
		Project: "p",
		Services: map[string]YAMLService{
			"api": {Path: "./api"},
		},
		Deps: map[string]YAMLDependency{
			"postgres": {Image: "postgres:16"},
		},
	}

	if _, err := YAMLToDeps(cfg); err != nil {
		t.Errorf("unexpected error on non-colliding config: %v", err)
	}
}

func TestYAMLToDeps_MultipleCollisionsReported(t *testing.T) {
	cfg := &RaiozConfig{
		Project: "p",
		Services: map[string]YAMLService{
			"postgres": {Path: "./pg"},
			"redis":    {Path: "./r"},
			"api":      {Path: "./api"},
		},
		Deps: map[string]YAMLDependency{
			"postgres": {Image: "postgres:16"},
			"redis":    {Image: "redis:7"},
		},
	}

	_, err := YAMLToDeps(cfg)
	if err == nil {
		t.Fatal("expected error on multi-name collision")
	}
	msg := err.Error()
	if !strings.Contains(msg, "postgres") || !strings.Contains(msg, "redis") {
		t.Errorf("error should list both colliding names, got: %v", err)
	}
}
