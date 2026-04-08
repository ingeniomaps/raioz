package upcase

import (
	"bytes"
	"strings"
	"testing"

	"raioz/internal/config"
)

func TestShowDryRunSummary(t *testing.T) {
	initI18nUp(t)

	deps := &config.Deps{
		SchemaVersion: "1.0",
		Network:       config.NetworkConfig{Name: "test-net"},
		Project:       config.Project{Name: "my-project"},
		Services: map[string]config.Service{
			"api": {Source: config.SourceConfig{Kind: "image", Image: "nginx"}},
			"web": {Source: config.SourceConfig{Kind: "git", Repo: "repo", Branch: "main", Path: "services/web"}},
		},
		Infra: map[string]config.InfraEntry{
			"postgres": {Inline: &config.Infra{Image: "postgres", Tag: "15"}},
		},
	}

	var buf bytes.Buffer
	uc := NewUseCase(&Dependencies{})
	uc.Out = &buf

	uc.showDryRunSummary(deps, []string{"web"})

	output := buf.String()

	if !strings.Contains(output, "my-project") {
		t.Errorf("should show project name\ngot: %s", output)
	}
	if !strings.Contains(output, "test-net") {
		t.Errorf("should show network\ngot: %s", output)
	}
	if !strings.Contains(output, "api") {
		t.Errorf("should show image service 'api'\ngot: %s", output)
	}
	if !strings.Contains(output, "web") {
		t.Errorf("should show git service 'web'\ngot: %s", output)
	}
	if !strings.Contains(output, "postgres") {
		t.Errorf("should show infra 'postgres'\ngot: %s", output)
	}
	if !strings.Contains(output, "web") {
		t.Errorf("should show override 'web'\ngot: %s", output)
	}
}

func TestShowDryRunSummaryNoOptionals(t *testing.T) {
	initI18nUp(t)

	deps := &config.Deps{
		SchemaVersion: "1.0",
		Network:       config.NetworkConfig{Name: "net"},
		Project:       config.Project{Name: "minimal"},
		Services: map[string]config.Service{
			"svc": {Source: config.SourceConfig{Kind: "image", Image: "img"}},
		},
		Infra: map[string]config.InfraEntry{},
	}

	var buf bytes.Buffer
	uc := NewUseCase(&Dependencies{})
	uc.Out = &buf

	uc.showDryRunSummary(deps, nil)

	output := buf.String()
	if !strings.Contains(output, "minimal") {
		t.Errorf("should show project\ngot: %s", output)
	}
	// Should NOT contain override or infra lines
	if strings.Contains(output, "Override") || strings.Contains(output, "override") {
		t.Errorf("should not show overrides when none applied\ngot: %s", output)
	}
}
