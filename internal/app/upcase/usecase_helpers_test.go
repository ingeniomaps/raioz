package upcase

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"raioz/internal/config"
	"raioz/internal/detect"
	"raioz/internal/domain/interfaces"
	"raioz/internal/mocks"
	"raioz/internal/state"
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
	deps := &config.Deps{
		Services: map[string]config.Service{},
		Infra: map[string]config.InfraEntry{
			"postgres": {Inline: &config.Infra{Image: "postgres", Tag: "16"}},
			"redis":    {Inline: &config.Infra{Image: "redis"}},
		},
	}

	results := detectRuntimes(context.Background(), deps)

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	for name, r := range results {
		if r.Runtime != detect.RuntimeImage {
			t.Errorf("%s: runtime = %q, want image", name, r.Runtime)
		}
	}
}

func TestDetectRuntimesServiceWithEmptyPath(t *testing.T) {
	deps := &config.Deps{
		Services: map[string]config.Service{
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
	deps := &config.Deps{
		Services: map[string]config.Service{
			"api": {Source: config.SourceConfig{Path: dir}},
		},
	}
	results := detectRuntimes(context.Background(), deps)
	if _, ok := results["api"]; !ok {
		t.Error("expected api in results")
	}
}

// --- buildEndpoints ------------------------------------------------------------

func TestBuildEndpointsDocker(t *testing.T) {
	deps := &config.Deps{
		Project: config.Project{Name: "app"},
		Services: map[string]config.Service{
			"web": {Docker: &config.DockerConfig{Ports: []string{"3000:3000"}}},
		},
	}
	detections := DetectionMap{
		"web": {Runtime: detect.RuntimeCompose, Port: 3000},
	}

	got := buildEndpoints(deps, detections, nil)
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
	deps := &config.Deps{
		Project: config.Project{Name: "app"},
		Services: map[string]config.Service{
			"api": {},
		},
	}
	detections := DetectionMap{
		"api": {Runtime: detect.RuntimeGo, Port: 8080},
	}
	got := buildEndpoints(deps, detections, nil)
	ep := got["api"]
	if ep.Host != "localhost" {
		t.Errorf("Host = %q, want localhost", ep.Host)
	}
	if ep.Port != 8080 {
		t.Errorf("Port = %d, want 8080", ep.Port)
	}
}

func TestBuildEndpointsInfraPortOverride(t *testing.T) {
	deps := &config.Deps{
		Project: config.Project{Name: "app"},
		Infra: map[string]config.InfraEntry{
			"db": {Inline: &config.Infra{Image: "postgres", Ports: []string{"5433:5432"}}},
		},
	}
	detections := DetectionMap{
		"db": {Runtime: detect.RuntimeImage, Port: 5432},
	}
	got := buildEndpoints(deps, detections, nil)
	ep := got["db"]
	if ep.Port != 5433 {
		t.Errorf("Port = %d, want 5433 (config override)", ep.Port)
	}
}

// --- checkServicesRunning ------------------------------------------------------

func TestCheckServicesRunningEmpty(t *testing.T) {
	initI18nUp(t)
	uc := NewUseCase(&Dependencies{})
	deps := &config.Deps{
		Services: map[string]config.Service{},
		Infra:    map[string]config.InfraEntry{},
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
	deps := &config.Deps{
		Services: map[string]config.Service{"api": {}},
	}
	oldDeps := &config.Deps{}
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
	deps := &config.Deps{Services: map[string]config.Service{"api": {}}}
	ws := &workspace.Workspace{Root: "/tmp/foo"}
	changes := []state.ConfigChange{{}}
	ok, err := uc.checkServicesRunning(context.Background(), deps, ws, changes, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Error("should return false when there are changes")
	}
}

// --- processState --------------------------------------------------------------

func TestProcessStateLoadError(t *testing.T) {
	initI18nUp(t)
	uc := NewUseCase(&Dependencies{
		StateManager: &mocks.MockStateManager{
			LoadFunc: func(ws *workspace.Workspace) (*config.Deps, error) {
				return nil, errors.New("load failed")
			},
		},
	})
	deps := &config.Deps{}
	ws := &workspace.Workspace{Root: "/tmp"}
	_, _, _, _, err := uc.processState(context.Background(), deps, ws, "cfg.json")
	if err == nil {
		t.Error("expected error when state load fails")
	}
}

func TestProcessStateCompareError(t *testing.T) {
	initI18nUp(t)
	uc := NewUseCase(&Dependencies{
		StateManager: &mocks.MockStateManager{
			LoadFunc: func(ws *workspace.Workspace) (*config.Deps, error) {
				return &config.Deps{}, nil
			},
			CompareDepsFunc: func(oldDeps, newDeps *config.Deps) ([]state.ConfigChange, error) {
				return nil, errors.New("compare failed")
			},
		},
	})
	deps := &config.Deps{}
	ws := &workspace.Workspace{Root: "/tmp"}
	_, _, _, _, err := uc.processState(context.Background(), deps, ws, "cfg.json")
	if err == nil {
		t.Error("expected error when compare fails")
	}
}

func TestProcessStateSuccess(t *testing.T) {
	initI18nUp(t)
	uc := NewUseCase(&Dependencies{
		StateManager: &mocks.MockStateManager{
			LoadFunc: func(ws *workspace.Workspace) (*config.Deps, error) {
				return &config.Deps{Project: config.Project{Name: "p"}}, nil
			},
			CompareDepsFunc: func(oldDeps, newDeps *config.Deps) ([]state.ConfigChange, error) {
				return []state.ConfigChange{{Field: "x", OldValue: "a", NewValue: "b"}}, nil
			},
			FormatChangesFunc: func(changes []state.ConfigChange) string { return "diff" },
		},
	})
	deps := &config.Deps{}
	ws := &workspace.Workspace{Root: "/tmp"}
	oldDeps, changes, added, assisted, err := uc.processState(context.Background(), deps, ws, "cfg.json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if oldDeps == nil {
		t.Error("expected non-nil oldDeps")
	}
	if len(changes) != 1 {
		t.Errorf("expected 1 change, got %d", len(changes))
	}
	if added == nil {
		t.Error("addedServices should be non-nil (even if empty)")
	}
	if assisted == nil {
		t.Error("assistedServicesMap should be non-nil")
	}
}

// --- isLocalProject ------------------------------------------------------------

func TestIsLocalProjectOutsideRaioz(t *testing.T) {
	// Use temp dir and set RAIOZ_HOME to a different temp dir
	raiozHome := t.TempDir()
	projectDir := t.TempDir()

	t.Setenv("RAIOZ_HOME", raiozHome)

	configPath := filepath.Join(projectDir, ".raioz.json")
	isLocal, dir, err := isLocalProject(configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !isLocal {
		t.Errorf("project outside RAIOZ_HOME should be local, got isLocal=%v", isLocal)
	}
	if dir != projectDir {
		t.Errorf("dir = %q, want %q", dir, projectDir)
	}
}

func TestIsLocalProjectInsideWorkspaces(t *testing.T) {
	raiozHome := t.TempDir()
	t.Setenv("RAIOZ_HOME", raiozHome)

	wsDir := filepath.Join(raiozHome, "workspaces", "myws")
	if err := os.MkdirAll(wsDir, 0755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(wsDir, ".raioz.json")

	isLocal, _, err := isLocalProject(configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if isLocal {
		t.Error("project inside workspaces should not be local")
	}
}

func TestIsLocalProjectInsideServices(t *testing.T) {
	raiozHome := t.TempDir()
	t.Setenv("RAIOZ_HOME", raiozHome)

	svcDir := filepath.Join(raiozHome, "services", "web")
	if err := os.MkdirAll(svcDir, 0755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(svcDir, ".raioz.json")

	isLocal, _, err := isLocalProject(configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if isLocal {
		t.Error("project inside services dir should not be local")
	}
}

func TestGetBaseDirForLocalCheckEnv(t *testing.T) {
	t.Setenv("RAIOZ_HOME", "/custom/home")
	got, err := getBaseDirForLocalCheck()
	if err != nil {
		t.Fatal(err)
	}
	if got != "/custom/home" {
		t.Errorf("got %q, want /custom/home", got)
	}
}

func TestGetBaseDirForLocalCheckDefault(t *testing.T) {
	t.Setenv("RAIOZ_HOME", "")
	got, err := getBaseDirForLocalCheck()
	if err != nil {
		return // Ignore if we can't get home dir
	}
	if !filepath.IsAbs(got) {
		t.Errorf("expected absolute path, got %q", got)
	}
}

// --- checkLocalProjectHealth ---------------------------------------------------

func TestCheckLocalProjectHealthNoCommand(t *testing.T) {
	ok, err := checkLocalProjectHealth(context.Background(), "/tmp", "")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("empty command should return false")
	}
}

func TestCheckLocalProjectHealthTrueCommand(t *testing.T) {
	// 'true' always exits 0
	ok, err := checkLocalProjectHealth(context.Background(), "/tmp", "true")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("true command should be healthy")
	}
}

func TestCheckLocalProjectHealthFalseCommand(t *testing.T) {
	ok, err := checkLocalProjectHealth(context.Background(), "/tmp", "false")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("false command should not be healthy")
	}
}

// --- executeLocalProjectCommand ------------------------------------------------

func TestExecuteLocalProjectCommandEmpty(t *testing.T) {
	initI18nUp(t)
	err := executeLocalProjectCommand(context.Background(), "/tmp", "", "dev")
	if err != nil {
		t.Errorf("empty command should be no-op, got error: %v", err)
	}
}

func TestExecuteLocalProjectCommandTrue(t *testing.T) {
	initI18nUp(t)
	err := executeLocalProjectCommand(context.Background(), t.TempDir(), "true", "dev")
	if err != nil {
		t.Errorf("true should succeed, got: %v", err)
	}
}

func TestExecuteLocalProjectCommandFail(t *testing.T) {
	initI18nUp(t)
	err := executeLocalProjectCommand(context.Background(), t.TempDir(), "false", "dev")
	if err == nil {
		t.Error("false should fail")
	}
}

// --- saveProjectCommandState ---------------------------------------------------

func TestSaveProjectCommandStateSuccess(t *testing.T) {
	initI18nUp(t)
	var updated *state.ProjectState
	uc := NewUseCase(&Dependencies{
		StateManager: &mocks.MockStateManager{
			UpdateProjectStateFunc: func(name string, ps *state.ProjectState) error {
				updated = ps
				return nil
			},
		},
	})
	deps := &config.Deps{Project: config.Project{Name: "p"}}
	err := uc.saveProjectCommandState(context.Background(), deps, "/proj")
	if err != nil {
		t.Fatal(err)
	}
	if updated == nil {
		t.Fatal("update was not called")
	}
	if updated.Name != "p" {
		t.Errorf("Name = %q", updated.Name)
	}
	if updated.Workspace != "/proj" {
		t.Errorf("Workspace = %q", updated.Workspace)
	}
}

func TestSaveProjectCommandStateError(t *testing.T) {
	initI18nUp(t)
	uc := NewUseCase(&Dependencies{
		StateManager: &mocks.MockStateManager{
			UpdateProjectStateFunc: func(name string, ps *state.ProjectState) error {
				return errors.New("fail")
			},
		},
	})
	err := uc.saveProjectCommandState(context.Background(), &config.Deps{
		Project: config.Project{Name: "p"},
	}, "/proj")
	if err == nil {
		t.Error("expected error")
	}
}

// --- generateEnvFilesFromTemplates ---------------------------------------------

func TestGenerateEnvFilesFromTemplates(t *testing.T) {
	initI18nUp(t)

	callCount := 0
	uc := NewUseCase(&Dependencies{
		Workspace: &mocks.MockWorkspaceManager{
			GetServicePathFunc: func(ws *workspace.Workspace, name string, svc config.Service) string {
				return "/path/" + name
			},
		},
		EnvManager: &mocks.MockEnvManager{
			GenerateEnvFromTemplateFunc: func(
				ws *workspace.Workspace, d *config.Deps, name, path string,
				svc config.Service, projEnv, projDir string,
			) error {
				callCount++
				return nil
			},
		},
	})
	disabled := false
	deps := &config.Deps{
		Services: map[string]config.Service{
			"git1":     {Source: config.SourceConfig{Kind: "git"}},
			"git2":     {Source: config.SourceConfig{Kind: "git"}},
			"image":    {Source: config.SourceConfig{Kind: "image"}},
			"disabled": {Source: config.SourceConfig{Kind: "git"}, Enabled: &disabled},
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
			GetServicePathFunc: func(ws *workspace.Workspace, name string, svc config.Service) string {
				return "/p/" + name
			},
		},
		EnvManager: &mocks.MockEnvManager{
			GenerateEnvFromTemplateFunc: func(
				ws *workspace.Workspace, d *config.Deps, name, path string,
				svc config.Service, pe, pd string,
			) error {
				callCount++
				return errors.New("template err")
			},
		},
	})
	deps := &config.Deps{
		Services: map[string]config.Service{
			"a": {Source: config.SourceConfig{Kind: "git"}},
			"b": {Source: config.SourceConfig{Kind: "git"}},
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
