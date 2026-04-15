package upcase

import (
	"context"
	stderrors "errors"
	"testing"

	"raioz/internal/config"
	"raioz/internal/mocks"
	"raioz/internal/state"
	"raioz/internal/workspace"
)

// --- processGitRepos: branch change detection --------------------------------

func TestProcessGitReposBranchChange(t *testing.T) {
	initI18nUp(t)

	updateCalled := false
	uc := NewUseCase(&Dependencies{
		Workspace: &mocks.MockWorkspaceManager{
			GetServicePathFunc: func(ws *workspace.Workspace, n string, svc config.Service) string {
				return "/svc/" + n
			},
			GetServiceDirFunc: func(ws *workspace.Workspace, svc config.Service) string {
				return t.TempDir()
			},
			GetComposePathFunc: func(ws *workspace.Workspace) string { return "" },
		},
		GitRepository: &mocks.MockGitRepository{
			UpdateReposIfBranchChangedFunc: func(
				ctx context.Context,
				resolver func(string, config.Service) string,
				oldDeps, newDeps *config.Deps,
			) error {
				updateCalled = true
				return nil
			},
			EnsureRepoWithForceFunc: func(src config.SourceConfig, baseDir string, force bool) error {
				return nil
			},
			IsReadonlyFunc: func(src config.SourceConfig) bool {
				return false
			},
		},
		StateManager: &mocks.MockStateManager{
			GetServicePreferenceFunc: func(ws *workspace.Workspace, name string) (*state.ServicePreference, error) {
				return nil, nil
			},
		},
	})

	oldDeps := &config.Deps{
		Project: config.Project{Name: "p"},
		Services: map[string]config.Service{
			"api": {Source: config.SourceConfig{Kind: "git", Repo: "r", Branch: "main"}},
		},
	}
	newDeps := &config.Deps{
		Project: config.Project{Name: "p"},
		Services: map[string]config.Service{
			"api": {Source: config.SourceConfig{Kind: "git", Repo: "r", Branch: "develop"}},
		},
	}

	err := uc.processGitRepos(
		context.Background(), newDeps, &workspace.Workspace{Root: "/t"},
		oldDeps, false, "/proj",
	)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !updateCalled {
		t.Error("UpdateReposIfBranchChanged should be called when branches differ")
	}
}

func TestProcessGitReposBranchChangeError(t *testing.T) {
	initI18nUp(t)

	uc := NewUseCase(&Dependencies{
		Workspace: &mocks.MockWorkspaceManager{
			GetServicePathFunc: func(ws *workspace.Workspace, n string, svc config.Service) string {
				return "/svc/" + n
			},
		},
		GitRepository: &mocks.MockGitRepository{
			UpdateReposIfBranchChangedFunc: func(
				ctx context.Context,
				resolver func(string, config.Service) string,
				oldDeps, newDeps *config.Deps,
			) error {
				return stderrors.New("git error")
			},
		},
	})

	oldDeps := &config.Deps{
		Project: config.Project{Name: "p"},
		Services: map[string]config.Service{
			"api": {Source: config.SourceConfig{Kind: "git", Repo: "r", Branch: "main"}},
		},
	}
	newDeps := &config.Deps{
		Project: config.Project{Name: "p"},
		Services: map[string]config.Service{
			"api": {Source: config.SourceConfig{Kind: "git", Repo: "r", Branch: "develop"}},
		},
	}

	err := uc.processGitRepos(
		context.Background(), newDeps, &workspace.Workspace{Root: "/t"},
		oldDeps, false, "/proj",
	)
	if err == nil {
		t.Error("expected error from branch update failure")
	}
}

func TestProcessGitReposCloneSuccess(t *testing.T) {
	initI18nUp(t)

	cloneCalled := false
	uc := NewUseCase(&Dependencies{
		Workspace: &mocks.MockWorkspaceManager{
			GetServicePathFunc: func(ws *workspace.Workspace, n string, svc config.Service) string {
				return "/svc/" + n
			},
			GetServiceDirFunc: func(ws *workspace.Workspace, svc config.Service) string {
				return t.TempDir() // New dir → repo doesn't exist
			},
			GetComposePathFunc: func(ws *workspace.Workspace) string { return "" },
		},
		GitRepository: &mocks.MockGitRepository{
			EnsureRepoWithForceFunc: func(src config.SourceConfig, baseDir string, force bool) error {
				cloneCalled = true
				return nil
			},
			IsReadonlyFunc: func(src config.SourceConfig) bool {
				return false
			},
		},
		StateManager: &mocks.MockStateManager{
			GetServicePreferenceFunc: func(ws *workspace.Workspace, name string) (*state.ServicePreference, error) {
				return nil, nil
			},
		},
	})

	deps := &config.Deps{
		Project: config.Project{Name: "p"},
		Services: map[string]config.Service{
			"web": {Source: config.SourceConfig{Kind: "git", Repo: "https://github.com/x/y", Branch: "main"}},
		},
	}

	err := uc.processGitRepos(
		context.Background(), deps, &workspace.Workspace{Root: "/t"},
		nil, false, "/proj",
	)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !cloneCalled {
		t.Error("EnsureRepoWithForce should be called for git service")
	}
}

func TestProcessGitReposCloneError(t *testing.T) {
	initI18nUp(t)

	uc := NewUseCase(&Dependencies{
		Workspace: &mocks.MockWorkspaceManager{
			GetServicePathFunc: func(ws *workspace.Workspace, n string, svc config.Service) string {
				return "/svc/" + n
			},
			GetServiceDirFunc: func(ws *workspace.Workspace, svc config.Service) string {
				return t.TempDir()
			},
			GetComposePathFunc: func(ws *workspace.Workspace) string { return "" },
		},
		GitRepository: &mocks.MockGitRepository{
			EnsureRepoWithForceFunc: func(src config.SourceConfig, baseDir string, force bool) error {
				return stderrors.New("clone failed")
			},
			IsReadonlyFunc: func(src config.SourceConfig) bool {
				return false
			},
		},
		StateManager: &mocks.MockStateManager{
			GetServicePreferenceFunc: func(ws *workspace.Workspace, name string) (*state.ServicePreference, error) {
				return nil, nil
			},
		},
	})

	deps := &config.Deps{
		Project: config.Project{Name: "p"},
		Services: map[string]config.Service{
			"web": {Source: config.SourceConfig{Kind: "git", Repo: "repo", Branch: "main"}},
		},
	}

	err := uc.processGitRepos(
		context.Background(), deps, &workspace.Workspace{Root: "/t"},
		nil, false, "/proj",
	)
	if err == nil {
		t.Error("expected clone error")
	}
}

func TestProcessGitReposSkipsDisabledService(t *testing.T) {
	initI18nUp(t)

	cloneCalled := false
	disabled := false
	uc := NewUseCase(&Dependencies{
		Workspace: &mocks.MockWorkspaceManager{
			GetServicePathFunc: func(ws *workspace.Workspace, n string, svc config.Service) string {
				return "/svc/" + n
			},
			GetServiceDirFunc: func(ws *workspace.Workspace, svc config.Service) string {
				return t.TempDir()
			},
		},
		GitRepository: &mocks.MockGitRepository{
			EnsureRepoWithForceFunc: func(src config.SourceConfig, baseDir string, force bool) error {
				cloneCalled = true
				return nil
			},
		},
	})

	deps := &config.Deps{
		Project: config.Project{Name: "p"},
		Services: map[string]config.Service{
			"web": {
				Source:  config.SourceConfig{Kind: "git", Repo: "repo", Branch: "main"},
				Enabled: &disabled,
			},
		},
	}

	err := uc.processGitRepos(
		context.Background(), deps, &workspace.Workspace{Root: "/t"},
		nil, false, "/proj",
	)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if cloneCalled {
		t.Error("disabled service should be skipped")
	}
}

func TestProcessGitReposReadonlyExistingRepo(t *testing.T) {
	initI18nUp(t)

	cloneCalled := false
	uc := NewUseCase(&Dependencies{
		Workspace: &mocks.MockWorkspaceManager{
			GetServicePathFunc: func(ws *workspace.Workspace, n string, svc config.Service) string {
				return "/svc/" + n
			},
			GetServiceDirFunc: func(ws *workspace.Workspace, svc config.Service) string {
				// Return existing dir → repo existed
				dir := t.TempDir()
				return dir
			},
			GetComposePathFunc: func(ws *workspace.Workspace) string { return "" },
		},
		GitRepository: &mocks.MockGitRepository{
			EnsureRepoWithForceFunc: func(src config.SourceConfig, baseDir string, force bool) error {
				cloneCalled = true
				return nil
			},
			IsReadonlyFunc: func(src config.SourceConfig) bool {
				return true
			},
		},
		StateManager: &mocks.MockStateManager{
			GetServicePreferenceFunc: func(ws *workspace.Workspace, name string) (*state.ServicePreference, error) {
				return nil, nil
			},
		},
	})

	deps := &config.Deps{
		Project: config.Project{Name: "p"},
		Services: map[string]config.Service{
			"lib": {Source: config.SourceConfig{Kind: "git", Repo: "repo", Branch: "main"}},
		},
	}

	err := uc.processGitRepos(
		context.Background(), deps, &workspace.Workspace{Root: "/t"},
		nil, false, "/proj",
	)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !cloneCalled {
		t.Error("should still call EnsureRepoWithForce for readonly")
	}
}

func TestProcessGitReposForceReclone(t *testing.T) {
	initI18nUp(t)

	forceUsed := false
	uc := NewUseCase(&Dependencies{
		Workspace: &mocks.MockWorkspaceManager{
			GetServicePathFunc: func(ws *workspace.Workspace, n string, svc config.Service) string {
				return "/svc/" + n
			},
			GetServiceDirFunc: func(ws *workspace.Workspace, svc config.Service) string {
				return t.TempDir()
			},
			GetComposePathFunc: func(ws *workspace.Workspace) string { return "" },
		},
		GitRepository: &mocks.MockGitRepository{
			EnsureRepoWithForceFunc: func(src config.SourceConfig, baseDir string, force bool) error {
				forceUsed = force
				return nil
			},
			IsReadonlyFunc: func(src config.SourceConfig) bool {
				return false
			},
		},
		StateManager: &mocks.MockStateManager{
			GetServicePreferenceFunc: func(ws *workspace.Workspace, name string) (*state.ServicePreference, error) {
				return nil, nil
			},
		},
	})

	deps := &config.Deps{
		Project: config.Project{Name: "p"},
		Services: map[string]config.Service{
			"web": {Source: config.SourceConfig{Kind: "git", Repo: "repo", Branch: "main"}},
		},
	}

	err := uc.processGitRepos(
		context.Background(), deps, &workspace.Workspace{Root: "/t"},
		nil, true, "/proj",
	)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !forceUsed {
		t.Error("force flag should be passed through")
	}
}
