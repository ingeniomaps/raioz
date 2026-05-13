package upcase

import (
	"context"
	stderrors "errors"
	"testing"

	"raioz/internal/domain/models"
	"raioz/internal/mocks"
	"raioz/internal/workspace"
)

// --- processGitRepos: branch change detection --------------------------------

func TestProcessGitReposBranchChange(t *testing.T) {
	initI18nUp(t)

	updateCalled := false
	uc := NewUseCase(&Dependencies{
		Workspace: &mocks.MockWorkspaceManager{
			GetServicePathFunc: func(ws *workspace.Workspace, n string, svc models.Service) string {
				return "/svc/" + n
			},
			GetServiceDirFunc: func(ws *workspace.Workspace, svc models.Service) string {
				return t.TempDir()
			},
			GetComposePathFunc: func(ws *workspace.Workspace) string { return "" },
		},
		GitRepository: &mocks.MockGitRepository{
			UpdateReposIfBranchChangedFunc: func(
				ctx context.Context,
				resolver func(string, models.Service) string,
				oldDeps, newDeps *models.Deps,
			) error {
				updateCalled = true
				return nil
			},
			EnsureRepoWithForceFunc: func(src models.SourceConfig, baseDir string, force bool) error {
				return nil
			},
			IsReadonlyFunc: func(src models.SourceConfig) bool {
				return false
			},
		},
		StateManager: &mocks.MockStateManager{
			GetServicePreferenceFunc: func(ws *workspace.Workspace, name string) (*models.ServicePreference, error) {
				return nil, nil
			},
		},
	})

	oldDeps := &models.Deps{
		Project: models.Project{Name: "p"},
		Services: map[string]models.Service{
			"api": {Source: models.SourceConfig{Kind: "git", Repo: "r", Branch: "main"}},
		},
	}
	newDeps := &models.Deps{
		Project: models.Project{Name: "p"},
		Services: map[string]models.Service{
			"api": {Source: models.SourceConfig{Kind: "git", Repo: "r", Branch: "develop"}},
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
			GetServicePathFunc: func(ws *workspace.Workspace, n string, svc models.Service) string {
				return "/svc/" + n
			},
		},
		GitRepository: &mocks.MockGitRepository{
			UpdateReposIfBranchChangedFunc: func(
				ctx context.Context,
				resolver func(string, models.Service) string,
				oldDeps, newDeps *models.Deps,
			) error {
				return stderrors.New("git error")
			},
		},
	})

	oldDeps := &models.Deps{
		Project: models.Project{Name: "p"},
		Services: map[string]models.Service{
			"api": {Source: models.SourceConfig{Kind: "git", Repo: "r", Branch: "main"}},
		},
	}
	newDeps := &models.Deps{
		Project: models.Project{Name: "p"},
		Services: map[string]models.Service{
			"api": {Source: models.SourceConfig{Kind: "git", Repo: "r", Branch: "develop"}},
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
			GetServicePathFunc: func(ws *workspace.Workspace, n string, svc models.Service) string {
				return "/svc/" + n
			},
			GetServiceDirFunc: func(ws *workspace.Workspace, svc models.Service) string {
				return t.TempDir() // New dir → repo doesn't exist
			},
			GetComposePathFunc: func(ws *workspace.Workspace) string { return "" },
		},
		GitRepository: &mocks.MockGitRepository{
			EnsureRepoWithForceFunc: func(src models.SourceConfig, baseDir string, force bool) error {
				cloneCalled = true
				return nil
			},
			IsReadonlyFunc: func(src models.SourceConfig) bool {
				return false
			},
		},
		StateManager: &mocks.MockStateManager{
			GetServicePreferenceFunc: func(ws *workspace.Workspace, name string) (*models.ServicePreference, error) {
				return nil, nil
			},
		},
	})

	deps := &models.Deps{
		Project: models.Project{Name: "p"},
		Services: map[string]models.Service{
			"web": {Source: models.SourceConfig{Kind: "git", Repo: "https://github.com/x/y", Branch: "main"}},
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
			GetServicePathFunc: func(ws *workspace.Workspace, n string, svc models.Service) string {
				return "/svc/" + n
			},
			GetServiceDirFunc: func(ws *workspace.Workspace, svc models.Service) string {
				return t.TempDir()
			},
			GetComposePathFunc: func(ws *workspace.Workspace) string { return "" },
		},
		GitRepository: &mocks.MockGitRepository{
			EnsureRepoWithForceFunc: func(src models.SourceConfig, baseDir string, force bool) error {
				return stderrors.New("clone failed")
			},
			IsReadonlyFunc: func(src models.SourceConfig) bool {
				return false
			},
		},
		StateManager: &mocks.MockStateManager{
			GetServicePreferenceFunc: func(ws *workspace.Workspace, name string) (*models.ServicePreference, error) {
				return nil, nil
			},
		},
	})

	deps := &models.Deps{
		Project: models.Project{Name: "p"},
		Services: map[string]models.Service{
			"web": {Source: models.SourceConfig{Kind: "git", Repo: "repo", Branch: "main"}},
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
			GetServicePathFunc: func(ws *workspace.Workspace, n string, svc models.Service) string {
				return "/svc/" + n
			},
			GetServiceDirFunc: func(ws *workspace.Workspace, svc models.Service) string {
				return t.TempDir()
			},
		},
		GitRepository: &mocks.MockGitRepository{
			EnsureRepoWithForceFunc: func(src models.SourceConfig, baseDir string, force bool) error {
				cloneCalled = true
				return nil
			},
		},
	})

	deps := &models.Deps{
		Project: models.Project{Name: "p"},
		Services: map[string]models.Service{
			"web": {
				Source:  models.SourceConfig{Kind: "git", Repo: "repo", Branch: "main"},
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
			GetServicePathFunc: func(ws *workspace.Workspace, n string, svc models.Service) string {
				return "/svc/" + n
			},
			GetServiceDirFunc: func(ws *workspace.Workspace, svc models.Service) string {
				// Return existing dir → repo existed
				dir := t.TempDir()
				return dir
			},
			GetComposePathFunc: func(ws *workspace.Workspace) string { return "" },
		},
		GitRepository: &mocks.MockGitRepository{
			EnsureRepoWithForceFunc: func(src models.SourceConfig, baseDir string, force bool) error {
				cloneCalled = true
				return nil
			},
			IsReadonlyFunc: func(src models.SourceConfig) bool {
				return true
			},
		},
		StateManager: &mocks.MockStateManager{
			GetServicePreferenceFunc: func(ws *workspace.Workspace, name string) (*models.ServicePreference, error) {
				return nil, nil
			},
		},
	})

	deps := &models.Deps{
		Project: models.Project{Name: "p"},
		Services: map[string]models.Service{
			"lib": {Source: models.SourceConfig{Kind: "git", Repo: "repo", Branch: "main"}},
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
			GetServicePathFunc: func(ws *workspace.Workspace, n string, svc models.Service) string {
				return "/svc/" + n
			},
			GetServiceDirFunc: func(ws *workspace.Workspace, svc models.Service) string {
				return t.TempDir()
			},
			GetComposePathFunc: func(ws *workspace.Workspace) string { return "" },
		},
		GitRepository: &mocks.MockGitRepository{
			EnsureRepoWithForceFunc: func(src models.SourceConfig, baseDir string, force bool) error {
				forceUsed = force
				return nil
			},
			IsReadonlyFunc: func(src models.SourceConfig) bool {
				return false
			},
		},
		StateManager: &mocks.MockStateManager{
			GetServicePreferenceFunc: func(ws *workspace.Workspace, name string) (*models.ServicePreference, error) {
				return nil, nil
			},
		},
	})

	deps := &models.Deps{
		Project: models.Project{Name: "p"},
		Services: map[string]models.Service{
			"web": {Source: models.SourceConfig{Kind: "git", Repo: "repo", Branch: "main"}},
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
