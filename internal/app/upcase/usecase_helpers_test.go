package upcase

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"raioz/internal/domain/interfaces"
	"raioz/internal/domain/models"
	"raioz/internal/mocks"
	"raioz/internal/workspace"
)

// --- NewUseCase / out ---------------------------------------------------------

func TestNewUseCaseDefaults(t *testing.T) {
	uc := NewUseCase(&Dependencies{})
	if uc == nil {
		t.Fatal("NewUseCase returned nil")
	}
	if uc.deps == nil {
		t.Error("deps should not be nil")
	}
	if uc.Out == nil {
		t.Error("Out should default to os.Stdout")
	}
}

func TestUseCaseOutFallback(t *testing.T) {
	uc := NewUseCase(&Dependencies{})
	uc.Out = nil
	w := uc.out()
	if w == nil {
		t.Error("out() should never return nil")
	}
	if w != os.Stdout {
		t.Error("out() should return os.Stdout when Out is nil")
	}
}

func TestUseCaseOutCustom(t *testing.T) {
	uc := NewUseCase(&Dependencies{})
	uc.Out = io.Discard
	if uc.out() != io.Discard {
		t.Error("out() should return the custom writer")
	}
}

// --- detectRuntimes ------------------------------------------------------------

func TestDetectRuntimesImages(t *testing.T) {
	deps := &models.Deps{
		Services: map[string]models.Service{},
		Infra: map[string]models.InfraEntry{
			"postgres": {Inline: &models.Infra{Image: "postgres", Tag: "16"}},
			"redis":    {Inline: &models.Infra{Image: "redis"}},
		},
	}

	results := detectRuntimes(context.Background(), deps)

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	for name, r := range results {
		if r.Runtime != models.RuntimeImage {
			t.Errorf("%s: runtime = %q, want image", name, r.Runtime)
		}
	}
}

func TestDetectRuntimesServiceWithEmptyPath(t *testing.T) {
	deps := &models.Deps{
		Services: map[string]models.Service{
			"api": {}, // empty path -> skipped
		},
	}
	results := detectRuntimes(context.Background(), deps)
	if _, ok := results["api"]; ok {
		t.Error("service with empty path should be skipped")
	}
}

func TestDetectRuntimesServiceWithPath(t *testing.T) {
	dir := t.TempDir()
	// Create a package.json so detection picks NPM
	if err := os.WriteFile(
		filepath.Join(dir, "package.json"),
		[]byte(`{"name":"x"}`), 0644,
	); err != nil {
		t.Fatal(err)
	}
	deps := &models.Deps{
		Services: map[string]models.Service{
			"api": {Source: models.SourceConfig{Path: dir}},
		},
	}
	results := detectRuntimes(context.Background(), deps)
	if _, ok := results["api"]; !ok {
		t.Error("expected api in results")
	}
}

// --- buildEndpoints ------------------------------------------------------------

func TestBuildEndpointsDocker(t *testing.T) {
	deps := &models.Deps{
		Project: models.Project{Name: "app"},
		Services: map[string]models.Service{
			"web": {Docker: &models.DockerConfig{Ports: []string{"3000:3000"}}},
		},
	}
	detections := DetectionMap{
		"web": {Runtime: models.RuntimeCompose, Port: 3000},
	}

	got := buildEndpoints(context.Background(), nil, deps, detections, nil)
	ep, ok := got["web"]
	if !ok {
		t.Fatal("missing web endpoint")
	}
	if ep.Port != 3000 {
		t.Errorf("Port = %d, want 3000", ep.Port)
	}
	// Host includes the naming prefix (default "raioz")
	if ep.Host == "" || ep.Host == "localhost" {
		t.Errorf("Docker endpoint should have container-style host, got %q", ep.Host)
	}
}

func TestBuildEndpointsHost(t *testing.T) {
	deps := &models.Deps{
		Project: models.Project{Name: "app"},
		Services: map[string]models.Service{
			"api": {},
		},
	}
	detections := DetectionMap{
		"api": {Runtime: models.RuntimeGo, Port: 8080},
	}
	got := buildEndpoints(context.Background(), nil, deps, detections, nil)
	ep := got["api"]
	if ep.Host != "localhost" {
		t.Errorf("Host = %q, want localhost", ep.Host)
	}
	if ep.Port != 8080 {
		t.Errorf("Port = %d, want 8080", ep.Port)
	}
}

func TestBuildEndpointsInfraPortOverride(t *testing.T) {
	deps := &models.Deps{
		Project: models.Project{Name: "app"},
		Infra: map[string]models.InfraEntry{
			"db": {Inline: &models.Infra{Image: "postgres", Ports: []string{"5433:5432"}}},
		},
	}
	detections := DetectionMap{
		"db": {Runtime: models.RuntimeImage, Port: 5432},
	}
	got := buildEndpoints(context.Background(), nil, deps, detections, nil)
	ep := got["db"]
	if ep.Port != 5433 {
		t.Errorf("Port = %d, want 5433 (config override)", ep.Port)
	}
}

// --- checkServicesRunning ------------------------------------------------------

func TestCheckServicesRunningEmpty(t *testing.T) {
	initI18nUp(t)
	uc := NewUseCase(&Dependencies{})
	deps := &models.Deps{
		Services: map[string]models.Service{},
		Infra:    map[string]models.InfraEntry{},
	}
	ws := &workspace.Workspace{Root: "/tmp/foo"}
	ok, err := uc.checkServicesRunning(context.Background(), deps, ws, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Error("should return false for empty services/infra")
	}
}

func TestCheckServicesRunningAllRunning(t *testing.T) {
	initI18nUp(t)
	uc := NewUseCase(&Dependencies{
		Workspace: &mocks.MockWorkspaceManager{
			GetComposePathFunc: func(ws *workspace.Workspace) string { return "/path/compose.yml" },
		},
		DockerRunner: &mocks.MockDockerRunner{
			AreServicesRunningFunc: func(composePath string, serviceNames []string) (bool, error) {
				return true, nil
			},
		},
	})
	deps := &models.Deps{
		Services: map[string]models.Service{"api": {}},
	}
	oldDeps := &models.Deps{}
	ws := &workspace.Workspace{Root: "/tmp/foo"}
	ok, err := uc.checkServicesRunning(context.Background(), deps, ws, nil, oldDeps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("should return true when all services running")
	}
}

func TestCheckServicesRunningWithChanges(t *testing.T) {
	initI18nUp(t)
	uc := NewUseCase(&Dependencies{})
	deps := &models.Deps{Services: map[string]models.Service{"api": {}}}
	ws := &workspace.Workspace{Root: "/tmp/foo"}
	changes := []models.ConfigChange{{}}
	ok, err := uc.checkServicesRunning(context.Background(), deps, ws, changes, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Error("should return false when there are changes")
	}
}

// --- processState --------------------------------------------------------------
//
// ADR-011 Phase 2 collapsed processState into a no-op. The drift detection it
// performed against the legacy snapshot is gone; raioz `up` is convergent so
// the loss is informational. Tests for the old branches (load error, compare
// error, success-with-changes) are removed; the new contract is "always
// returns nil oldDeps, nil changes, empty addedServices, empty assisted map".

func TestProcessStateNoOp(t *testing.T) {
	initI18nUp(t)
	uc := NewUseCase(&Dependencies{})
	deps := &models.Deps{}
	ws := &workspace.Workspace{Root: "/tmp"}
	oldDeps, changes, added, assisted, err := uc.processState(context.Background(), deps, ws, "cfg.json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if oldDeps != nil {
		t.Error("expected nil oldDeps after ADR-011 Phase 2")
	}
	if len(changes) != 0 {
		t.Errorf("expected zero changes, got %d", len(changes))
	}
	if added == nil {
		t.Error("addedServices should be non-nil (even if empty)")
	}
	if assisted == nil {
		t.Error("assistedServicesMap should be non-nil")
	}
}

// --- isLocalProject ------------------------------------------------------------

// --- checkLocalProjectHealth ---------------------------------------------------

// --- executeLocalProjectCommand ------------------------------------------------

// --- saveProjectCommandState ---------------------------------------------------

// --- generateEnvFilesFromTemplates ---------------------------------------------

func TestGenerateEnvFilesFromTemplates(t *testing.T) {
	initI18nUp(t)

	callCount := 0
	uc := NewUseCase(&Dependencies{
		Workspace: &mocks.MockWorkspaceManager{
			GetServicePathFunc: func(ws *workspace.Workspace, name string, svc models.Service) string {
				return "/path/" + name
			},
		},
		EnvManager: &mocks.MockEnvManager{
			GenerateEnvFromTemplateFunc: func(
				ws *workspace.Workspace, d *models.Deps, name, path string,
				svc models.Service, projEnv, projDir string,
			) error {
				callCount++
				return nil
			},
		},
	})
	disabled := false
	deps := &models.Deps{
		Services: map[string]models.Service{
			"git1":     {Source: models.SourceConfig{Kind: "git"}},
			"git2":     {Source: models.SourceConfig{Kind: "git"}},
			"image":    {Source: models.SourceConfig{Kind: "image"}},
			"disabled": {Source: models.SourceConfig{Kind: "git"}, Enabled: &disabled},
		},
	}
	ws := &workspace.Workspace{Root: "/tmp"}

	err := uc.generateEnvFilesFromTemplates(context.Background(), deps, ws, "/env", "/proj")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should have called only for git1 and git2 (not image, not disabled)
	if callCount != 2 {
		t.Errorf("GenerateEnvFromTemplate called %d times, want 2", callCount)
	}
}

func TestGenerateEnvFilesFromTemplatesErrorContinues(t *testing.T) {
	initI18nUp(t)
	callCount := 0
	uc := NewUseCase(&Dependencies{
		Workspace: &mocks.MockWorkspaceManager{
			GetServicePathFunc: func(ws *workspace.Workspace, name string, svc models.Service) string {
				return "/p/" + name
			},
		},
		EnvManager: &mocks.MockEnvManager{
			GenerateEnvFromTemplateFunc: func(
				ws *workspace.Workspace, d *models.Deps, name, path string,
				svc models.Service, pe, pd string,
			) error {
				callCount++
				return errors.New("template err")
			},
		},
	})
	deps := &models.Deps{
		Services: map[string]models.Service{
			"a": {Source: models.SourceConfig{Kind: "git"}},
			"b": {Source: models.SourceConfig{Kind: "git"}},
		},
	}
	ws := &workspace.Workspace{Root: "/tmp"}
	err := uc.generateEnvFilesFromTemplates(context.Background(), deps, ws, "/env", "/proj")
	if err != nil {
		t.Fatalf("should not propagate errors, got: %v", err)
	}
	if callCount != 2 {
		t.Errorf("expected 2 attempts, got %d", callCount)
	}
}

// --- acquireLock ---------------------------------------------------------------

type fakeLock struct {
	released bool
	relErr   error
}

func (f *fakeLock) Release() error { f.released = true; return f.relErr }

func TestAcquireLockSuccess(t *testing.T) {
	initI18nUp(t)
	l := &fakeLock{}
	uc := NewUseCase(&Dependencies{
		LockManager: &mocks.MockLockManager{
			AcquireFunc: func(ws *workspace.Workspace) (interfaces.Lock, error) {
				return l, nil
			},
		},
	})
	ws := &workspace.Workspace{Root: "/tmp"}
	inst, err := uc.acquireLock(context.Background(), ws)
	if err != nil {
		t.Fatal(err)
	}
	if inst == nil {
		t.Fatal("expected non-nil lock instance")
	}
	if err := inst.Release(); err != nil {
		t.Error("unexpected release error")
	}
	if !l.released {
		t.Error("lock was not released")
	}
}

func TestAcquireLockError(t *testing.T) {
	initI18nUp(t)
	uc := NewUseCase(&Dependencies{
		LockManager: &mocks.MockLockManager{
			AcquireFunc: func(ws *workspace.Workspace) (interfaces.Lock, error) {
				return nil, errors.New("busy")
			},
		},
	})
	ws := &workspace.Workspace{Root: "/tmp"}
	_, err := uc.acquireLock(context.Background(), ws)
	if err == nil {
		t.Error("expected error")
	}
}

func TestLockInstanceReleaseNilLock(t *testing.T) {
	inst := &LockInstance{ctx: context.Background()}
	if err := inst.Release(); err != nil {
		t.Errorf("nil lock should release cleanly, got: %v", err)
	}
}

func TestLockInstanceReleaseError(t *testing.T) {
	l := &fakeLock{relErr: errors.New("boom")}
	inst := &LockInstance{lock: l, ctx: context.Background()}
	if err := inst.Release(); err == nil {
		t.Error("expected release error")
	}
}
