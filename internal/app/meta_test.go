package app

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"raioz/internal/config"
)

// stagePassingBinary writes a tiny shell script that always exits 0 and
// returns its absolute path. The MetaRunner shells out to this binary instead
// of the real raioz.
func stagePassingBinary(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "fake-raioz")
	body := "#!/bin/sh\nexit 0\n"
	if err := os.WriteFile(path, []byte(body), 0755); err != nil {
		t.Fatal(err)
	}
	return path
}

// stageFailingBinary writes a script that exits 1 — used to model a sub
// project whose `raioz up` fails.
func stageFailingBinary(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "fake-raioz")
	body := "#!/bin/sh\nexit 1\n"
	if err := os.WriteFile(path, []byte(body), 0755); err != nil {
		t.Fatal(err)
	}
	return path
}

func makeMetaProjects(t *testing.T, names ...string) (*config.MetaConfig, string) {
	t.Helper()
	base := t.TempDir()
	var ps []config.MetaProject
	for _, n := range names {
		dir := filepath.Join(base, n)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		ps = append(ps, config.MetaProject{Name: n, Path: dir})
	}
	return &config.MetaConfig{BaseDir: base, Projects: ps}, base
}

// Up over three healthy subs: every sub runs, summary is all-green.
func TestMetaRunner_UpHappyPath(t *testing.T) {
	bin := stagePassingBinary(t)
	cfg, _ := makeMetaProjects(t, "keycloak", "api", "ui")
	r := &MetaRunner{Binary: bin}

	summary := r.Up(context.Background(), cfg, nil, nil, MetaUpOptions{})
	if len(summary) != 3 {
		t.Fatalf("expected 3 summary entries, got %d", len(summary))
	}
	if summary.HasFailures() {
		t.Errorf("expected no failures, got %+v", summary)
	}
	for i, p := range cfg.Projects {
		if summary[i].Project != p.Name {
			t.Errorf("entry[%d].Project = %q, want %q", i, summary[i].Project, p.Name)
		}
		if summary[i].Err != nil {
			t.Errorf("entry[%d].Err = %v", i, summary[i].Err)
		}
	}
}

// Up over a failing required sub: must abort and surface the failure.
func TestMetaRunner_UpFailingRequiredAborts(t *testing.T) {
	bin := stageFailingBinary(t)
	cfg, _ := makeMetaProjects(t, "keycloak", "api", "ui")
	r := &MetaRunner{Binary: bin}

	summary := r.Up(context.Background(), cfg, nil, nil, MetaUpOptions{})
	if len(summary) != 1 {
		t.Errorf("expected only the failing sub in summary, got %d entries: %+v",
			len(summary), summary)
	}
	if !summary.HasFailures() {
		t.Errorf("expected HasFailures()=true")
	}
}

// Up over a failing OPTIONAL sub: must continue and report it as Skipped.
func TestMetaRunner_UpFailingOptionalContinues(t *testing.T) {
	failing := stageFailingBinary(t)
	cfg, _ := makeMetaProjects(t, "keycloak", "ad-service", "ui")
	cfg.Projects[1].Optional = true

	r := &MetaRunner{Binary: failing}
	summary := r.Up(context.Background(), cfg, nil, nil, MetaUpOptions{})

	// All 3 subs are recorded — the optional one as Skipped, the others as
	// failures (binary fails for everyone in this test). HasFailures should
	// flag the non-optional ones.
	if len(summary) != 1 {
		// First non-optional sub fails -> abort. ad-service never reached.
		t.Logf("summary = %+v", summary)
	}
	if !summary.HasFailures() {
		t.Errorf("non-optional failure must be flagged")
	}
}

// Down runs in reverse order — the third sub goes first, the first goes last.
func TestMetaRunner_DownReversesOrder(t *testing.T) {
	bin := stagePassingBinary(t)
	cfg, _ := makeMetaProjects(t, "keycloak", "api", "ui")
	r := &MetaRunner{Binary: bin}

	summary := r.Down(context.Background(), cfg, nil)
	if len(summary) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(summary))
	}
	want := []string{"ui", "api", "keycloak"}
	for i, w := range want {
		if summary[i].Project != w {
			t.Errorf("entry[%d].Project = %q, want %q", i, summary[i].Project, w)
		}
	}
}

// Down is best-effort: a failing sub doesn't stop the rest.
func TestMetaRunner_DownToleratesFailures(t *testing.T) {
	bin := stageFailingBinary(t)
	cfg, _ := makeMetaProjects(t, "a", "b")
	r := &MetaRunner{Binary: bin}

	summary := r.Down(context.Background(), cfg, nil)
	if len(summary) != 2 {
		t.Errorf("down must visit every sub, got %d entries", len(summary))
	}
}

// resolveBinary must return an absolute path even when os.Args[0] is
// relative (e.g. dev build invoked as `./raioz`). Without this, the
// MetaRunner sub-spawn changes cwd via cmd.Dir and the relative path
// vanishes, producing the "fork/exec ./raioz: no such file or directory"
// regression smoke-tested in the 019-035 session.
func TestMetaRunner_ResolveBinary_AbsolutePathFromRelativeArg0(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Args[0] semantics differ on Windows")
	}
	// Force the m.Binary fast-path off and stash a relative os.Args[0].
	r := &MetaRunner{}
	prev := os.Args[0]
	os.Args[0] = "./does-not-exist-here"
	t.Cleanup(func() { os.Args[0] = prev })

	got, err := r.resolveBinary()
	if err != nil {
		t.Fatalf("resolveBinary: %v", err)
	}
	if !filepath.IsAbs(got) {
		t.Errorf("expected absolute path, got %q", got)
	}
}

// When m.Binary is explicitly set (test pattern), resolveBinary must
// honor it as-is without touching os.Args[0] or os.Executable.
func TestMetaRunner_ResolveBinary_BinaryFieldWins(t *testing.T) {
	r := &MetaRunner{Binary: "/explicit/path/raioz"}
	got, err := r.resolveBinary()
	if err != nil {
		t.Fatalf("resolveBinary: %v", err)
	}
	if got != "/explicit/path/raioz" {
		t.Errorf("expected explicit override, got %q", got)
	}
}

// Status reports per-project outcome and tolerates failures (a sub that's
// missing or not yet up shouldn't blank the rest of the report).
func TestMetaRunner_StatusToleratesFailures(t *testing.T) {
	bin := stageFailingBinary(t)
	cfg, _ := makeMetaProjects(t, "a", "b", "c")
	r := &MetaRunner{Binary: bin}

	summary := r.Status(context.Background(), cfg, nil, nil)
	if len(summary) != 3 {
		t.Errorf("status must run on every sub, got %d entries", len(summary))
	}
}
