package upcase

import (
	"bytes"
	"strings"
	"testing"

	"raioz/internal/domain/models"
)

func TestShowDryRunSummary(t *testing.T) {
	initI18nUp(t)

	deps := &models.Deps{
		SchemaVersion: "1.0",
		Network:       models.NetworkConfig{Name: "test-net"},
		Project:       models.Project{Name: "my-project"},
		Services: map[string]models.Service{
			"api": {Source: models.SourceConfig{Kind: "image", Image: "nginx"}},
			"web": {Source: models.SourceConfig{Kind: "git", Repo: "repo", Branch: "main", Path: "services/web"}},
		},
		Infra: map[string]models.InfraEntry{
			"postgres": {Inline: &models.Infra{Image: "postgres", Tag: "15"}},
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

	deps := &models.Deps{
		SchemaVersion: "1.0",
		Network:       models.NetworkConfig{Name: "net"},
		Project:       models.Project{Name: "minimal"},
		Services: map[string]models.Service{
			"svc": {Source: models.SourceConfig{Kind: "image", Image: "img"}},
		},
		Infra: map[string]models.InfraEntry{},
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
