package app

import (
	"context"
	"errors"
	"slices"
	"testing"

	"raioz/internal/mocks"
	"raioz/internal/naming"
	"raioz/internal/refcount"
)

// cleanUCWithProbe builds a CleanUseCase whose DockerRunner reports the named
// projects as running (everything else stopped), and swaps composeDownProject
// for the supplied stub so the GC runs without a Docker daemon.
func cleanUCWithProbe(
	t *testing.T,
	probe func(ctx context.Context, workspace, project string) (bool, error),
	teardown func(context.Context, string) ([]byte, error),
) *CleanUseCase {
	t.Helper()
	orig := composeDownProject
	if teardown != nil {
		composeDownProject = teardown
	}
	t.Cleanup(func() {
		composeDownProject = orig
		naming.SetPrefix("")
	})
	return &CleanUseCase{deps: &Dependencies{
		DockerRunner: &mocks.MockDockerRunner{IsProjectActiveFunc: probe},
	}}
}

func liveSet(live ...string) func(context.Context, string, string) (bool, error) {
	set := map[string]bool{}
	for _, p := range live {
		set[p] = true
	}
	return func(_ context.Context, _, project string) (bool, error) {
		return set[project], nil
	}
}

// A stale ref (project not running) is pruned while a live consumer's ref
// is kept — and because a live consumer remains, the dep is NOT torn down.
func TestPruneStaleSharedRefs_DropsDeadKeepsLive(t *testing.T) {
	t.Setenv("RAIOZ_HOME", t.TempDir())
	uc := cleanUCWithProbe(t, liveSet("api"), func(context.Context, string) ([]byte, error) {
		t.Fatal("teardown must not run while a live consumer remains")
		return nil, nil
	})

	mustAddRef(t, "conorbi", "loki", "bff") // stale (not in live set)
	mustAddRef(t, "conorbi", "loki", "api") // live

	actions := uc.pruneStaleSharedRefs(context.Background(), []string{"conorbi"}, false)

	refs, err := refcount.Refs("conorbi", "loki")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(refs, []string{"api"}) {
		t.Errorf("loki refs after prune = %v, want [api]", refs)
	}
	if len(actions) != 1 {
		t.Errorf("actions = %v, want exactly one prune line", actions)
	}
}

// When every referencing project is gone, the dep is an orphan: its refs are
// cleared and its compose project is torn down.
func TestPruneStaleSharedRefs_TearsDownOrphan(t *testing.T) {
	t.Setenv("RAIOZ_HOME", t.TempDir())
	var torn string
	uc := cleanUCWithProbe(t, liveSet(), func(_ context.Context, projName string) ([]byte, error) {
		torn = projName
		return nil, nil
	})

	mustAddRef(t, "conorbi", "loki", "bff") // only ref, and it's dead

	uc.pruneStaleSharedRefs(context.Background(), []string{"conorbi"}, false)

	refs, err := refcount.Refs("conorbi", "loki")
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 0 {
		t.Errorf("loki refs after orphan prune = %v, want empty", refs)
	}
	if want := naming.SharedDepComposeProjectName("loki"); torn != want {
		t.Errorf("torn compose project = %q, want %q", torn, want)
	}
}

// Dry-run classifies and reports but writes nothing and tears nothing down.
func TestPruneStaleSharedRefs_DryRunIsReadOnly(t *testing.T) {
	t.Setenv("RAIOZ_HOME", t.TempDir())
	uc := cleanUCWithProbe(t, liveSet(), func(context.Context, string) ([]byte, error) {
		t.Fatal("dry-run must not tear anything down")
		return nil, nil
	})

	mustAddRef(t, "conorbi", "loki", "bff")

	actions := uc.pruneStaleSharedRefs(context.Background(), []string{"conorbi"}, true)

	refs, err := refcount.Refs("conorbi", "loki")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(refs, []string{"bff"}) {
		t.Errorf("dry-run mutated refcount: %v, want [bff] untouched", refs)
	}
	if len(actions) != 2 { // pruned line + would-teardown line
		t.Errorf("dry-run actions = %v, want prune + would-teardown", actions)
	}
}

// A probe that errors must be treated as "live": the ref is conservatively
// kept rather than pruned on an answer we could not get (ADR-050 direction).
func TestPruneStaleSharedRefs_ProbeErrorKeepsRef(t *testing.T) {
	t.Setenv("RAIOZ_HOME", t.TempDir())
	uc := cleanUCWithProbe(t, func(context.Context, string, string) (bool, error) {
		return false, errors.New("docker daemon unreachable")
	}, func(context.Context, string) ([]byte, error) {
		t.Fatal("must not tear down when liveness is unknown")
		return nil, nil
	})

	mustAddRef(t, "conorbi", "loki", "bff")

	actions := uc.pruneStaleSharedRefs(context.Background(), []string{"conorbi"}, false)

	refs, err := refcount.Refs("conorbi", "loki")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(refs, []string{"bff"}) {
		t.Errorf("probe error pruned a ref: %v, want [bff] kept", refs)
	}
	if len(actions) != 0 {
		t.Errorf("actions = %v, want none on probe failure", actions)
	}
}

func mustAddRef(t *testing.T, ws, dep, project string) {
	t.Helper()
	if err := refcount.AddRef(ws, dep, project); err != nil {
		t.Fatalf("AddRef(%s,%s,%s): %v", ws, dep, project, err)
	}
}
