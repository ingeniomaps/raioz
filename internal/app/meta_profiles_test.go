package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"raioz/internal/config"
)

func TestShouldIncludeMetaProject(t *testing.T) {
	cases := []struct {
		name     string
		project  config.MetaProject
		active   []string
		expected bool
	}{
		{
			name:     "always-on (empty Profiles) with no active",
			project:  config.MetaProject{Profiles: nil},
			active:   nil,
			expected: true,
		},
		{
			name:     "always-on (empty Profiles) ignores any active set",
			project:  config.MetaProject{Profiles: nil},
			active:   []string{"edge", "ops"},
			expected: true,
		},
		{
			name:     "tagged project skipped when no profile active",
			project:  config.MetaProject{Profiles: []string{"edge"}},
			active:   nil,
			expected: false,
		},
		{
			name:     "tagged project included on exact match",
			project:  config.MetaProject{Profiles: []string{"edge"}},
			active:   []string{"edge"},
			expected: true,
		},
		{
			name:     "tagged project included via any-match",
			project:  config.MetaProject{Profiles: []string{"edge", "validation"}},
			active:   []string{"ops", "validation"},
			expected: true,
		},
		{
			name:     "tagged project skipped when active list misses every profile",
			project:  config.MetaProject{Profiles: []string{"edge"}},
			active:   []string{"ops"},
			expected: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := shouldIncludeMetaProject(tc.project, tc.active)
			if got != tc.expected {
				t.Errorf("shouldIncludeMetaProject(%v, %v) = %v, want %v",
					tc.project.Profiles, tc.active, got, tc.expected)
			}
		})
	}
}

// stageMetaWithProfiles builds a MetaConfig where one project is opt-in
// via Profiles=["edge"] and another is always-on. The fake-raioz binary
// is a passing shell script that writes its name to a counter file we
// read back to verify which projects were invoked.
func stageMetaWithProfiles(t *testing.T) (*config.MetaConfig, string, string) {
	t.Helper()
	base := t.TempDir()
	if err := os.MkdirAll(filepath.Join(base, "always"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(base, "edge"), 0o755); err != nil {
		t.Fatal(err)
	}

	counter := filepath.Join(base, "calls.log")
	binDir := t.TempDir()
	binPath := filepath.Join(binDir, "fake-raioz")
	// Use $PWD so each invocation appends its working dir to the log,
	// proving which project triggered the call.
	body := "#!/bin/sh\necho \"$PWD\" >>\"" + counter + "\"\nexit 0\n"
	if err := os.WriteFile(binPath, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}

	cfg := &config.MetaConfig{
		Workspace: "test-ws",
		BaseDir:   base,
		Projects: []config.MetaProject{
			{Name: "always", Path: filepath.Join(base, "always")},
			{Name: "edge", Path: filepath.Join(base, "edge"), Profiles: []string{"edge"}},
		},
	}
	return cfg, binPath, counter
}

func TestMetaRunner_Up_WithoutProfiles_SkipsTaggedProject(t *testing.T) {
	cfg, bin, counter := stageMetaWithProfiles(t)
	r := &MetaRunner{
		Binary: bin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
	summary := r.Up(context.Background(), cfg, nil, nil)

	if len(summary) != 1 {
		t.Errorf("expected 1 invocation, got %d (%+v)", len(summary), summary)
	}
	if summary[0].Project != "always" {
		t.Errorf("expected only 'always' invoked, got %q", summary[0].Project)
	}
	mustHaveOnly(t, counter, "always")
}

func TestMetaRunner_Up_WithMatchingProfile_IncludesTagged(t *testing.T) {
	cfg, bin, counter := stageMetaWithProfiles(t)
	r := &MetaRunner{Binary: bin, Stdout: os.Stdout, Stderr: os.Stderr}

	summary := r.Up(context.Background(), cfg, nil, []string{"edge"})

	if len(summary) != 2 {
		t.Errorf("expected 2 invocations, got %d (%+v)", len(summary), summary)
	}
	mustHaveBoth(t, counter, "always", "edge")
}

func TestMetaRunner_Down_IgnoresProfile_BringsDownEverything(t *testing.T) {
	cfg, bin, counter := stageMetaWithProfiles(t)
	r := &MetaRunner{Binary: bin, Stdout: os.Stdout, Stderr: os.Stderr}

	// Even though we pass no profiles, Down must reach both projects so
	// nothing strands. Symmetry with the "you can't half-leave services
	// up" invariant.
	summary := r.Down(context.Background(), cfg, nil)
	if len(summary) != 2 {
		t.Errorf("expected 2 invocations, got %d (%+v)", len(summary), summary)
	}
	mustHaveBoth(t, counter, "always", "edge")
}

func mustHaveOnly(t *testing.T, counter, name string) {
	t.Helper()
	b, err := os.ReadFile(counter)
	if err != nil {
		t.Fatalf("read counter: %v", err)
	}
	s := string(b)
	if !strings.Contains(s, name) {
		t.Errorf("expected %q in counter, got %q", name, s)
	}
	for _, other := range []string{"edge", "always", "ops"} {
		if other == name {
			continue
		}
		if strings.Contains(s, "/"+other+"\n") {
			t.Errorf("counter unexpectedly contains %q: %q", other, s)
		}
	}
}

func mustHaveBoth(t *testing.T, counter string, names ...string) {
	t.Helper()
	b, err := os.ReadFile(counter)
	if err != nil {
		t.Fatalf("read counter: %v", err)
	}
	s := string(b)
	for _, n := range names {
		if !strings.Contains(s, "/"+n+"\n") {
			t.Errorf("expected %q in counter, got %q", n, s)
		}
	}
}
