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
// regression for issue 069: a shared dep with a still-live sibling consumer
// must survive this project's down. The reconcile drops the leaving
// project's ref, the keep-alive check sees the sibling's ref, and the
// teardown is skipped (no compose-down shell-out for that dep).
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

	// The sibling "api" still has a live container in the workspace.
	prevList, prevLabel := listContainersByLabelsFn, getContainerLabelFn
	t.Cleanup(func() {
		listContainersByLabelsFn, getContainerLabelFn = prevList, prevLabel
	})
	listContainersByLabelsFn = func(_ context.Context, _ map[string]string) []string {
		return []string{"c-api"}
	}
	getContainerLabelFn = func(_ context.Context, _, _ string) (string, error) {
		return "api", nil
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

func TestAnyLive(t *testing.T) {
	cases := []struct {
		name string
		refs []string
		live []string
		want bool
	}{
		{"empty refs", nil, []string{"a"}, false},
		{"live match", []string{"a", "b"}, []string{"b"}, true},
		{"only stale refs", []string{"ghost"}, []string{"a"}, false},
		{"no live at all", []string{"a"}, nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := anyLive(tc.refs, tc.live); got != tc.want {
				t.Errorf("anyLive(%v, %v) = %v, want %v", tc.refs, tc.live, got, tc.want)
			}
		})
	}
}

func TestLiveProjectsInWorkspace(t *testing.T) {
	prevList := listContainersByLabelsFn
	prevLabel := getContainerLabelFn
	t.Cleanup(func() {
		listContainersByLabelsFn = prevList
		getContainerLabelFn = prevLabel
	})

	// Containers: two belong to sibling "api", one to a shared dep (no
	// project label), one to the current project (must be excluded).
	listContainersByLabelsFn = func(_ context.Context, _ map[string]string) []string {
		return []string{"c-api-1", "c-api-2", "c-shareddep", "c-self"}
	}
	labels := map[string]string{
		"c-api-1":     "api",
		"c-api-2":     "api", // duplicate project — must dedupe
		"c-shareddep": "",    // shared dep, no project label
		"c-self":      "observability",
	}
	getContainerLabelFn = func(_ context.Context, name, _ string) (string, error) {
		return labels[name], nil
	}

	live := liveProjectsInWorkspace(context.Background(), "conorbi", "observability")
	if !slices.Equal(live, []string{"api"}) {
		t.Errorf("live = %v, want [api] (deduped, self + shared-dep excluded)", live)
	}
}

func TestLiveProjectsInWorkspace_NoWorkspace(t *testing.T) {
	if live := liveProjectsInWorkspace(context.Background(), "", "p"); live != nil {
		t.Errorf("no workspace should yield nil, got %v", live)
	}
}

// Guard: the bulk teardown decision must use the active-prefix shared-dep
// predicate, not a literal. Sanity-checks the wiring assumption that a
// workspace makes deps shared.
func TestIsSharedDep_WorkspaceMakesShared(t *testing.T) {
	naming.SetPrefix("conorbi")
	t.Cleanup(func() { naming.SetPrefix("") })
	if !naming.IsSharedDep("") {
		t.Error("with a workspace prefix, an unnamed dep must be shared")
	}
}
