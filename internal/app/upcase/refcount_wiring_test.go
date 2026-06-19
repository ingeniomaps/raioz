package upcase

import (
	"context"
	"slices"
	"testing"

	"raioz/internal/domain/models"
	"raioz/internal/naming"
	"raioz/internal/refcount"
)

// TestRegisterSharedDepRefs asserts that `up` records a reference
// only for shared deps (workspace-scoped or name-overridden), leaving
// per-project deps untracked.
func TestRegisterSharedDepRefs(t *testing.T) {
	t.Setenv("RAIOZ_HOME", t.TempDir())
	naming.SetPrefix("conorbi")
	t.Cleanup(func() { naming.SetPrefix("") })

	deps := &models.Deps{
		Workspace: "conorbi",
		Project:   models.Project{Name: "observability"},
		Infra: map[string]models.InfraEntry{
			"loki":   {Inline: &models.Infra{Image: "grafana/loki:3"}},
			"jaeger": {Inline: &models.Infra{Image: "jaegertracing/all-in-one:1"}},
		},
	}

	registerSharedDepRefs(context.Background(), deps, []string{"loki", "jaeger"})

	for _, dep := range []string{"loki", "jaeger"} {
		refs, err := refcount.Refs("conorbi", dep)
		if err != nil {
			t.Fatalf("Refs(%s): %v", dep, err)
		}
		if !slices.Equal(refs, []string{"observability"}) {
			t.Errorf("%s refs = %v, want [observability]", dep, refs)
		}
	}
}

// TestRegisterSharedDepRefs_NoWorkspaceSkipsPlainDep verifies that without a
// workspace and without a name override, a dep is not shared and gets no
// reference recorded.
func TestRegisterSharedDepRefs_NoWorkspaceSkipsPlainDep(t *testing.T) {
	t.Setenv("RAIOZ_HOME", t.TempDir())
	naming.SetPrefix("") // no workspace
	t.Cleanup(func() { naming.SetPrefix("") })

	deps := &models.Deps{
		Project: models.Project{Name: "solo"},
		Infra: map[string]models.InfraEntry{
			"redis": {Inline: &models.Infra{Image: "redis:7"}},
		},
	}

	registerSharedDepRefs(context.Background(), deps, []string{"redis"})

	refs, err := refcount.Refs("", "redis")
	if err != nil {
		t.Fatalf("Refs: %v", err)
	}
	if len(refs) != 0 {
		t.Errorf("plain dep should not be tracked, got refs = %v", refs)
	}
}
