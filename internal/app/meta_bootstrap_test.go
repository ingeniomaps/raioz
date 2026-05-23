package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"raioz/internal/config"
	"raioz/internal/domain/interfaces"
	"raioz/internal/domain/models"
)

// fakeCloner is a MetaCloner stand-in that records every call and lets the
// test drive success / failure per project. Avoids shelling out to git for
// unit-level coverage.
type fakeCloner struct {
	calls []models.SourceConfig
	// fail maps RelPath -> error to return. Absence in the map = success.
	fail map[string]error
}

func (f *fakeCloner) clone(src models.SourceConfig, baseDir string) error {
	f.calls = append(f.calls, src)
	if err, bad := f.fail[src.Path]; bad {
		return err
	}
	// Create the dir so a subsequent stat sees it.
	if err := os.MkdirAll(filepath.Join(baseDir, src.Path), 0o755); err != nil {
		return err
	}
	return nil
}

func newBootstrapCfg(t *testing.T, projects ...config.MetaProject) (*config.MetaConfig, string) {
	t.Helper()
	base := t.TempDir()
	return &config.MetaConfig{BaseDir: base, Projects: projects}, base
}

func TestBootstrapMeta_SkipsLocalProjects(t *testing.T) {
	cfg, _ := newBootstrapCfg(t, config.MetaProject{
		Name: "api", RelPath: "api", Mode: config.MetaModeLocal,
	})
	fc := &fakeCloner{}
	m := &MetaRunner{}

	results, err := m.bootstrapMeta(context.Background(), cfg, fc.clone, recordRemote(t))

	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("local-only run must produce no bootstrap entries, got %d", len(results))
	}
	if len(fc.calls) != 0 {
		t.Errorf("local-only run must not invoke cloner, got %d calls", len(fc.calls))
	}
}

func TestBootstrapMeta_ClonesMissingProjects(t *testing.T) {
	cfg, _ := newBootstrapCfg(t, config.MetaProject{
		Name: "api", RelPath: "api", Mode: config.MetaModeClone,
		Git: "git@x:y/api.git", Branch: "develop", Auth: "gh",
	})
	fc := &fakeCloner{}
	m := &MetaRunner{}

	results, err := m.bootstrapMeta(context.Background(), cfg, fc.clone, recordRemote(t))

	if err != nil {
		t.Fatalf("clone failed: %v", err)
	}
	if len(fc.calls) != 1 {
		t.Fatalf("expected 1 clone call, got %d", len(fc.calls))
	}
	got := fc.calls[0]
	if got.Repo != "git@x:y/api.git" || got.Branch != "develop" || got.Auth != "gh" {
		t.Errorf("SourceConfig mismatch: %+v", got)
	}
	if cfg.Projects[0].Mode != config.MetaModeLocal {
		t.Errorf("post-clone Mode = %q, want %q",
			cfg.Projects[0].Mode, config.MetaModeLocal)
	}
	if len(results) != 1 || results[0].Err != nil {
		t.Errorf("expected one success entry, got %+v", results)
	}
}

func TestBootstrapMeta_OptionalCloneFailureSkipsAndContinues(t *testing.T) {
	cfg, _ := newBootstrapCfg(t,
		config.MetaProject{Name: "api", RelPath: "api", Mode: config.MetaModeClone, Git: "x", Optional: true},
		config.MetaProject{Name: "web", RelPath: "web", Mode: config.MetaModeClone, Git: "y"},
	)
	fc := &fakeCloner{fail: map[string]error{"api": errors.New("auth denied")}}
	m := &MetaRunner{}

	results, err := m.bootstrapMeta(context.Background(), cfg, fc.clone, recordRemote(t))

	if err != nil {
		t.Fatalf("optional failure must not abort: %v", err)
	}
	if cfg.Projects[0].Mode != config.MetaModeSkip {
		t.Errorf("optional-failed Mode = %q, want %q",
			cfg.Projects[0].Mode, config.MetaModeSkip)
	}
	if cfg.Projects[1].Mode != config.MetaModeLocal {
		t.Errorf("non-failed Mode = %q, want %q (clone succeeded)",
			cfg.Projects[1].Mode, config.MetaModeLocal)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 summary entries, got %d", len(results))
	}
	if !results[0].Skipped {
		t.Errorf("optional-failed entry must be Skipped=true")
	}
}

func TestBootstrapMeta_NonOptionalCloneFailureAborts(t *testing.T) {
	cfg, _ := newBootstrapCfg(t,
		config.MetaProject{Name: "api", RelPath: "api", Mode: config.MetaModeClone, Git: "x"},
		config.MetaProject{Name: "web", RelPath: "web", Mode: config.MetaModeClone, Git: "y"},
	)
	fc := &fakeCloner{fail: map[string]error{"api": errors.New("repo not found")}}
	m := &MetaRunner{}

	results, err := m.bootstrapMeta(context.Background(), cfg, fc.clone, recordRemote(t))

	if err == nil {
		t.Fatal("non-optional clone failure must abort with error")
	}
	if len(fc.calls) != 1 {
		t.Errorf("subsequent clones must NOT run after abort, got %d calls", len(fc.calls))
	}
	if len(results) != 1 {
		t.Errorf("results must contain only the failed entry, got %d", len(results))
	}
	if cfg.Projects[1].Mode != config.MetaModeClone {
		t.Errorf("web's Mode must stay Clone (untouched), got %q", cfg.Projects[1].Mode)
	}
}

func TestBootstrapMeta_NilCfgIsNoOp(t *testing.T) {
	m := &MetaRunner{}
	results, err := m.bootstrapMeta(context.Background(), nil, nil, nil)
	if err != nil || results != nil {
		t.Errorf("nil cfg must be a no-op, got results=%v err=%v", results, err)
	}
}

// recordRemote returns a RemoteRouteWriter that fails the test if it
// gets called — used as the writer for clone-only tests where no remote
// publish should happen.
func recordRemote(t *testing.T) RemoteRouteWriter {
	t.Helper()
	return func(workspace, project, _, _ string, _ []interfaces.ProxyRoute) error {
		t.Errorf("unexpected remote write: workspace=%q project=%q", workspace, project)
		return nil
	}
}

// fakeRemoteWriter captures every remote-route publish so tests can
// assert hostname/target without writing to the user's XDG state tree.
type fakeRemoteWriter struct {
	calls []remoteCall
	fail  error
}

type remoteCall struct {
	Workspace, Project, Domain, TLSMode string
	Routes                              []interfaces.ProxyRoute
}

func (f *fakeRemoteWriter) write(ws, proj, dom, tls string, r []interfaces.ProxyRoute) error {
	f.calls = append(f.calls, remoteCall{ws, proj, dom, tls, r})
	return f.fail
}

func TestBootstrapMeta_RemoteOnly_WritesRoute(t *testing.T) {
	cfg, _ := newBootstrapCfg(t, config.MetaProject{
		Name: "api", RelPath: "api", Path: "/abs/api",
		Mode:   config.MetaModeRemote,
		Remote: "https://api.staging.acme.dev",
	})
	cfg.Workspace = "acme"
	rw := &fakeRemoteWriter{}
	m := &MetaRunner{}

	results, err := m.bootstrapMeta(context.Background(), cfg, nil, rw.write)

	if err != nil {
		t.Fatalf("remote bootstrap failed: %v", err)
	}
	if len(rw.calls) != 1 {
		t.Fatalf("expected 1 remote write, got %d", len(rw.calls))
	}
	c := rw.calls[0]
	if c.Workspace != "acme" || c.Project != "api" {
		t.Errorf("writer args mismatch: %+v", c)
	}
	if len(c.Routes) != 1 || c.Routes[0].Hostname != "api" ||
		c.Routes[0].Target != "https://api.staging.acme.dev" {
		t.Errorf("route mismatch: %+v", c.Routes)
	}
	if cfg.Projects[0].Mode != config.MetaModeRemote {
		t.Errorf("Mode must stay Remote after publish, got %q", cfg.Projects[0].Mode)
	}
	if len(results) != 1 || results[0].Err != nil {
		t.Errorf("expected one success entry, got %+v", results)
	}
}

func TestBootstrapMeta_RemoteHostnameOverride(t *testing.T) {
	cfg, _ := newBootstrapCfg(t, config.MetaProject{
		Name: "api", RelPath: "client/api", Path: "/abs/client/api",
		Mode:           config.MetaModeRemote,
		Remote:         "https://api.staging.acme.dev",
		RemoteHostname: "client-api",
	})
	cfg.Workspace = "acme"
	rw := &fakeRemoteWriter{}
	m := &MetaRunner{}

	_, err := m.bootstrapMeta(context.Background(), cfg, nil, rw.write)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if rw.calls[0].Routes[0].Hostname != "client-api" {
		t.Errorf("Hostname = %q, want override %q",
			rw.calls[0].Routes[0].Hostname, "client-api")
	}
}

func TestBootstrapMeta_CloneFails_FallsBackToRemote(t *testing.T) {
	cfg, _ := newBootstrapCfg(t, config.MetaProject{
		Name: "api", RelPath: "api", Path: "/abs/api",
		Mode: config.MetaModeClone,
		Git:  "x", Remote: "https://api.staging.acme.dev",
	})
	cfg.Workspace = "acme"
	fc := &fakeCloner{fail: map[string]error{"api": errors.New("auth denied")}}
	rw := &fakeRemoteWriter{}
	m := &MetaRunner{}

	results, err := m.bootstrapMeta(context.Background(), cfg, fc.clone, rw.write)

	if err != nil {
		t.Fatalf("fallback should succeed: %v", err)
	}
	if cfg.Projects[0].Mode != config.MetaModeRemote {
		t.Errorf("post-fallback Mode = %q, want Remote", cfg.Projects[0].Mode)
	}
	if len(rw.calls) != 1 {
		t.Errorf("expected remote write after clone fallback, got %d calls", len(rw.calls))
	}
	// results: 1 clone-fail entry (marked Skipped — fallback took over)
	// + 1 remote-success entry.
	if len(results) != 2 {
		t.Fatalf("expected 2 summary entries (clone+remote), got %d", len(results))
	}
	if !results[0].Skipped {
		t.Errorf("clone-fail entry must be Skipped when fallback succeeded; got %+v", results[0])
	}
	if results[0].Err == nil {
		t.Errorf("clone-fail entry must still carry the error for log visibility")
	}
	if results[1].Err != nil || results[1].Skipped {
		t.Errorf("remote success entry malformed: %+v", results[1])
	}
	if MetaSummaryList(results).HasFailures() {
		t.Errorf("HasFailures must be false when clone failed but remote took over")
	}
}

func TestApplyForceRemote_UnknownProjectRejected(t *testing.T) {
	cfg := &config.MetaConfig{Projects: []config.MetaProject{
		{Name: "api", Remote: "https://x"},
	}}
	err := applyForceRemote(cfg, []string{"unknown"})
	if err == nil {
		t.Fatal("expected error for unknown project")
	}
}

func TestApplyForceRemote_MissingRemoteRejected(t *testing.T) {
	cfg := &config.MetaConfig{Projects: []config.MetaProject{
		{Name: "api"}, // no Remote URL
	}}
	err := applyForceRemote(cfg, []string{"api"})
	if err == nil {
		t.Fatal("expected error when forced project has no remote: URL")
	}
}

func TestApplyForceRemote_MutatesMode(t *testing.T) {
	cfg := &config.MetaConfig{Projects: []config.MetaProject{
		{Name: "api", Remote: "https://x", Mode: config.MetaModeLocal},
		{Name: "web", Mode: config.MetaModeLocal},
	}}
	if err := applyForceRemote(cfg, []string{"api"}); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if cfg.Projects[0].Mode != config.MetaModeRemote {
		t.Errorf("api Mode = %q, want Remote", cfg.Projects[0].Mode)
	}
	if cfg.Projects[1].Mode != config.MetaModeLocal {
		t.Errorf("web Mode = %q, want Local (untouched)", cfg.Projects[1].Mode)
	}
}
