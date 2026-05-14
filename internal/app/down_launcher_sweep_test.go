package app

import (
	"context"
	"path/filepath"
	"testing"

	"raioz/internal/domain/models"
)

// withKillOrphansHook swaps killOrphansByCwdFn for the test's duration,
// returning a slice the stub appends to so the test can verify which
// service path actually triggered a sweep.
func withKillOrphansHook(t *testing.T, killedFor map[string][]int) []string {
	t.Helper()
	var calls []string
	prev := killOrphansByCwdFn
	killOrphansByCwdFn = func(path string) []int {
		calls = append(calls, path)
		return killedFor[path]
	}
	t.Cleanup(func() { killOrphansByCwdFn = prev })
	return calls // length not stable; tests use the returned slice header
}

func TestSweepLauncherOrphans_KillsByResolvedPath(t *testing.T) {
	initI18nForTest(t)
	projectDir := "/home/u/proj"
	abs := filepath.Clean(filepath.Join(projectDir, "api"))
	var calls []string
	prev := killOrphansByCwdFn
	killOrphansByCwdFn = func(path string) []int {
		calls = append(calls, path)
		return []int{4242}
	}
	t.Cleanup(func() { killOrphansByCwdFn = prev })

	deps := &models.Deps{
		Services: map[string]models.Service{
			"api": {Source: models.SourceConfig{Path: "api"}},
		},
	}

	sweepLauncherOrphans(context.Background(), deps, projectDir, "api")

	if len(calls) != 1 || calls[0] != abs {
		t.Errorf("KillOrphansByCwd called with %v, want [%s]", calls, abs)
	}
}

func TestSweepLauncherOrphans_NilDeps(t *testing.T) {
	_ = withKillOrphansHook(t, nil)
	prev := killOrphansByCwdFn
	called := false
	killOrphansByCwdFn = func(string) []int { called = true; return nil }
	t.Cleanup(func() { killOrphansByCwdFn = prev })

	sweepLauncherOrphans(context.Background(), nil, "/proj", "api")

	if called {
		t.Error("nil deps must short-circuit; KillOrphansByCwd should not be called")
	}
}

func TestSweepLauncherOrphans_UnknownService(t *testing.T) {
	prev := killOrphansByCwdFn
	called := false
	killOrphansByCwdFn = func(string) []int { called = true; return nil }
	t.Cleanup(func() { killOrphansByCwdFn = prev })

	deps := &models.Deps{Services: map[string]models.Service{}}
	sweepLauncherOrphans(context.Background(), deps, "/proj", "missing")

	if called {
		t.Error("unknown service must short-circuit; sweep should not run")
	}
}

func TestSweepLauncherOrphans_EmptyPath(t *testing.T) {
	prev := killOrphansByCwdFn
	called := false
	killOrphansByCwdFn = func(string) []int { called = true; return nil }
	t.Cleanup(func() { killOrphansByCwdFn = prev })

	deps := &models.Deps{
		Services: map[string]models.Service{
			"api": {Source: models.SourceConfig{Path: ""}},
		},
	}
	sweepLauncherOrphans(context.Background(), deps, "/proj", "api")

	if called {
		t.Error("empty service path must short-circuit; sweep should not run")
	}
}

func TestAbsoluteServicePath_Variants(t *testing.T) {
	cases := []struct {
		name       string
		projectDir string
		raw        string
		want       string
	}{
		{"empty raw", "/proj", "", ""},
		{"empty projectDir", "", "api", ""},
		{"dot becomes projectDir", "/proj", ".", "/proj"},
		{"absolute raw kept", "/proj", "/elsewhere/api", "/elsewhere/api"},
		{"relative joins under projectDir", "/proj", "api", "/proj/api"},
		{"relative with dotdot cleaned", "/proj", "../sibling", "/sibling"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := absoluteServicePath(tc.projectDir, tc.raw)
			if got != tc.want {
				t.Errorf("absoluteServicePath(%q,%q) = %q, want %q",
					tc.projectDir, tc.raw, got, tc.want)
			}
		})
	}
}
