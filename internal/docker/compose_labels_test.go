package docker

import (
	"testing"

	"raioz/internal/domain/models"
	"raioz/internal/naming"
)

// Containers the legacy generator names must be findable by label, or
// `down`, `status` and the shared-proxy gate cannot see them (ADR-001).
func TestInlineInfraConfigStampsLabels(t *testing.T) {
	deps := &models.Deps{Project: models.Project{Name: "proj"}}

	got, err := buildInlineInfraConfig(
		"db", models.Infra{Image: "postgres", Tag: "16"}, deps,
		mkWorkspace(t.TempDir()), t.TempDir(), "raioz-net", "", false, map[string]string{},
	)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	labels, ok := got["labels"].(map[string]string)
	if !ok {
		t.Fatalf("labels missing: got %T", got["labels"])
	}
	if labels[naming.LabelManaged] != "true" {
		t.Errorf("%s = %q, want true", naming.LabelManaged, labels[naming.LabelManaged])
	}
	if labels[naming.LabelKind] != naming.KindDependency {
		t.Errorf("%s = %q, want %s", naming.LabelKind, labels[naming.LabelKind], naming.KindDependency)
	}
	if labels[naming.LabelProject] != "proj" {
		t.Errorf("project-scoped dep must carry its project, got %q", labels[naming.LabelProject])
	}
}

// With an explicit workspace the dep container is workspace-scoped and
// shared, so it must NOT carry a project label — otherwise one project's
// down rips it out from under its siblings (ADR-002).
func TestSharedInfraOmitsProjectLabel(t *testing.T) {
	deps := &models.Deps{Project: models.Project{Name: "proj"}}

	got, err := buildInlineInfraConfig(
		"db", models.Infra{Image: "postgres", Tag: "16"}, deps,
		mkWorkspace(t.TempDir()), t.TempDir(), "acme-net", "acme", true, map[string]string{},
	)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	labels := got["labels"].(map[string]string)
	if _, present := labels[naming.LabelProject]; present {
		t.Errorf("shared dep must omit the project label, got %q", labels[naming.LabelProject])
	}
	if labels[naming.LabelWorkspace] != "acme" {
		t.Errorf("%s = %q, want acme", naming.LabelWorkspace, labels[naming.LabelWorkspace])
	}
}

func TestServiceConfigStampsLabels(t *testing.T) {
	deps := &models.Deps{
		Project: models.Project{Name: "proj"},
		Services: map[string]models.Service{
			"api": {Source: models.SourceConfig{Kind: "image", Image: "nginx"},
				Docker: &models.DockerConfig{Mode: "prod"}},
		},
	}
	services := map[string]any{}

	err := addServiceToCompose(services, "api", deps.Services["api"], deps,
		mkWorkspace(t.TempDir()), t.TempDir(), "raioz-net", map[string]string{})
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	cfg := services["api"].(map[string]any)
	labels, ok := cfg["labels"].(map[string]string)
	if !ok {
		t.Fatalf("labels missing: got %T", cfg["labels"])
	}
	if labels[naming.LabelKind] != naming.KindService {
		t.Errorf("%s = %q, want %s", naming.LabelKind, labels[naming.LabelKind], naming.KindService)
	}
	if labels[naming.LabelService] != "api" {
		t.Errorf("%s = %q, want api", naming.LabelService, labels[naming.LabelService])
	}
}

// The external fragment belongs to the user: stamping must add to their
// labels, never replace them.
func TestMergeLabelsKeepsUserLabels(t *testing.T) {
	ours := map[string]string{naming.LabelManaged: "true"}

	t.Run("mapping", func(t *testing.T) {
		cfg := map[string]any{"labels": map[string]any{"team": "infra"}}
		mergeLabels(cfg, ours)
		got := cfg["labels"].(map[string]any)
		if got["team"] != "infra" || got[naming.LabelManaged] != "true" {
			t.Errorf("expected both labels, got %v", got)
		}
	})

	t.Run("list", func(t *testing.T) {
		cfg := map[string]any{"labels": []any{"team=infra"}}
		mergeLabels(cfg, ours)
		got := cfg["labels"].([]any)
		if len(got) != 2 || got[0] != "team=infra" {
			t.Errorf("expected the user entry kept and ours appended, got %v", got)
		}
	})

	t.Run("absent", func(t *testing.T) {
		cfg := map[string]any{}
		mergeLabels(cfg, ours)
		if got := cfg["labels"].(map[string]string); got[naming.LabelManaged] != "true" {
			t.Errorf("expected our labels, got %v", got)
		}
	})
}
