package app

import (
	"context"
	"slices"
	"testing"

	"raioz/internal/domain/models"
	"raioz/internal/naming"
	"raioz/internal/refcount"
)

// TestStopDependencyComposeProjects_KeepsAliveWhenReferenced is the
// regression for issue 069: a shared dep with another consumer must
// survive this project's down. The decision is taken from the refcount
// alone — no container scan — because a sibling that consumes only shared
// deps owns no project-labeled container and would be invisible to a scan.
func TestStopDependencyComposeProjects_KeepsAliveWhenReferenced(t *testing.T) {
	t.Setenv("RAIOZ_HOME", t.TempDir())
	naming.SetPrefix("conorbi")
	t.Cleanup(func() { naming.SetPrefix("") })

	// Both projects referenced loki at up time.
	if err := refcount.AddRef("conorbi", "loki", "observability"); err != nil {
		t.Fatal(err)
	}
	if err := refcount.AddRef("conorbi", "loki", "api"); err != nil {
		t.Fatal(err)
	}

	deps := &models.Deps{
		Workspace: "conorbi",
		Project:   models.Project{Name: "observability"},
		Infra: map[string]models.InfraEntry{
			"loki": {Inline: &models.Infra{Image: "grafana/loki:3"}},
		},
	}

	stopDependencyComposeProjects(context.Background(), deps, "observability", nil)

	// observability's ref is gone; api's remains so loki stays referenced.
	refs, err := refcount.Refs("conorbi", "loki")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(refs, []string{"api"}) {
		t.Errorf("loki refs after down = %v, want [api] (kept alive)", refs)
	}
}

// TestStopDependencyComposeProjects_LastConsumerDropsRef verifies the
// last consumer's down empties the ref set (the teardown then proceeds —
// the compose-down shell-out is exercised but harmless without Docker).
func TestStopDependencyComposeProjects_LastConsumerDropsRef(t *testing.T) {
	t.Setenv("RAIOZ_HOME", t.TempDir())
	naming.SetPrefix("conorbi")
	t.Cleanup(func() { naming.SetPrefix("") })

	if err := refcount.AddRef("conorbi", "loki", "observability"); err != nil {
		t.Fatal(err)
	}

	deps := &models.Deps{
		Workspace: "conorbi",
		Project:   models.Project{Name: "observability"},
		Infra: map[string]models.InfraEntry{
			"loki": {Inline: &models.Infra{Image: "grafana/loki:3"}},
		},
	}

	stopDependencyComposeProjects(context.Background(), deps, "observability", nil)

	refs, err := refcount.Refs("conorbi", "loki")
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 0 {
		t.Errorf("loki refs after last down = %v, want empty", refs)
	}
}

// Guard: the bulk teardown decision must use the active-prefix shared-dep
// predicate. Sanity-checks the wiring assumption that a workspace makes
// deps shared.
func TestIsSharedDep_WorkspaceMakesShared(t *testing.T) {
	naming.SetPrefix("conorbi")
	t.Cleanup(func() { naming.SetPrefix("") })
	if !naming.IsSharedDep("") {
		t.Error("with a workspace prefix, an unnamed dep must be shared")
	}
}
