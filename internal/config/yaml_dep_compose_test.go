package config

import (
	"strings"
	"testing"
)

func TestYAMLToDeps_DepWithCompose(t *testing.T) {
	cfg := &RaiozConfig{
		Project: "p",
		Deps: map[string]YAMLDependency{
			"postgres": {
				Compose: YAMLStringSlice{"./infra/postgres.yml"},
			},
		},
	}
	deps, err := YAMLToDeps(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := deps.Infra["postgres"].Inline.Compose
	if len(got) != 1 || got[0] != "./infra/postgres.yml" {
		t.Errorf("compose paths lost in bridge: %v", got)
	}
}

func TestYAMLToDeps_DepWithComposeMulti(t *testing.T) {
	cfg := &RaiozConfig{
		Project: "p",
		Deps: map[string]YAMLDependency{
			"postgres": {
				Compose: YAMLStringSlice{"./a.yml", "./b.yml"},
			},
		},
	}
	deps, _ := YAMLToDeps(cfg)
	if n := len(deps.Infra["postgres"].Inline.Compose); n != 2 {
		t.Errorf("expected 2 compose files, got %d", n)
	}
}

func TestYAMLToDeps_DepMustHaveImageOrCompose(t *testing.T) {
	cfg := &RaiozConfig{
		Project: "p",
		Deps: map[string]YAMLDependency{
			"postgres": {}, // neither image nor compose
		},
	}
	_, err := YAMLToDeps(cfg)
	if err == nil || !strings.Contains(err.Error(), "must declare either") {
		t.Errorf("expected 'must declare either' error, got %v", err)
	}
}

func TestYAMLToDeps_DepCannotHaveBothImageAndCompose(t *testing.T) {
	cfg := &RaiozConfig{
		Project: "p",
		Deps: map[string]YAMLDependency{
			"postgres": {
				Image:   "postgres:16",
				Compose: YAMLStringSlice{"./postgres.yml"},
			},
		},
	}
	_, err := YAMLToDeps(cfg)
	if err == nil || !strings.Contains(err.Error(), "both `image:` and `compose:`") {
		t.Errorf("expected conflict error, got %v", err)
	}
}

func TestYAMLToDeps_ImageOnlyStillWorks(t *testing.T) {
	cfg := &RaiozConfig{
		Project: "p",
		Deps: map[string]YAMLDependency{
			"postgres": {Image: "postgres:16"},
		},
	}
	deps, err := YAMLToDeps(cfg)
	if err != nil {
		t.Fatalf("image-only must still work, got %v", err)
	}
	if deps.Infra["postgres"].Inline.Image != "postgres" {
		t.Errorf("image lost, got %q", deps.Infra["postgres"].Inline.Image)
	}
	if len(deps.Infra["postgres"].Inline.Compose) != 0 {
		t.Error("compose should be empty for image-only dep")
	}
}
