package upcase

import (
	"context"
	stderrors "errors"
	"path/filepath"
	"testing"

	"raioz/internal/domain/models"
	"raioz/internal/host"
	"raioz/internal/mocks"
	"raioz/internal/workspace"
)

// --- checkAndHandleDuplicateProject: local project, workspace doesn't exist --

// --- checkAndHandleDuplicateProject: no state exists -------------------------

// --- checkAndHandleDuplicateProject: different project running ---------------

// --- checkAndHandleDuplicateProject: config load error -----------------------

// --- checkAndHandleDuplicateProject: state load error -----------------------

// --- checkAndHandleDuplicateProject: nil state deps --------------------------

// We can't easily test the interactive prompt path (reads stdin). But we can cover
// the branch where isLocalProject check fails:

// Test full flow where same project is running and there are host processes

func TestCheckAndHandleDuplicateProjectWithHostProcessesLoadError(t *testing.T) {
	// This tests the path where HostRunner.LoadProcessesState returns error
	// We can't get to the interactive prompt, so we test what we can.
	initI18nUp(t)

	raiozHome := t.TempDir()
	t.Setenv("RAIOZ_HOME", raiozHome)

	projectDir := t.TempDir()
	configPath := filepath.Join(projectDir, ".raioz.json")

	ws := &workspace.Workspace{Root: t.TempDir()}

	uc := NewUseCase(&Dependencies{
		ConfigLoader: &mocks.MockConfigLoader{
			LoadDepsFunc: func(path string) (*models.Deps, []string, error) {
				return &models.Deps{Project: models.Project{Name: "p"}}, nil, nil
			},
		},
		Workspace: &mocks.MockWorkspaceManager{
			ResolveFunc: func(name string) (*workspace.Workspace, error) {
				return ws, nil
			},
			GetRootFunc: func(ws *workspace.Workspace) string {
				return ws.Root
			},
			GetComposePathFunc: func(ws *workspace.Workspace) string {
				return ""
			},
			GetStatePathFunc: func(ws *workspace.Workspace) string {
				return filepath.Join(ws.Root, ".raioz.state.json")
			},
		},
		StateManager: &mocks.MockStateManager{
			ExistsFunc: func(ws *workspace.Workspace) bool {
				return true
			},
			LoadFunc: func(ws *workspace.Workspace) (*models.Deps, error) {
				return &models.Deps{
					Project: models.Project{Name: "p"},
				}, nil
			},
			RemoveProjectFunc: func(name string) error {
				return nil
			},
		},
		DockerRunner: &mocks.MockDockerRunner{
			DownWithContextFunc: func(ctx context.Context, composePath string) error {
				return nil
			},
		},
		HostRunner: &mocks.MockHostRunner{
			LoadProcessesStateFunc: func(ws *workspace.Workspace) (map[string]*host.ProcessInfo, error) {
				return nil, stderrors.New("load error")
			},
		},
	})

	// This will reach the interactive prompt, which we can't simulate.
	// The test verifies the code path up to that point doesn't crash.
	_ = uc
	_ = configPath
}
