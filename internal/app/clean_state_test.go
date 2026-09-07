package app

import (
	"context"
	"errors"
	"strings"
	"testing"

	"raioz/internal/domain/models"
	"raioz/internal/mocks"
)

func cleanUCWithState(
	t *testing.T, projects map[string]models.ProjectState,
	active func(ws, proj string) (bool, error), removed *[]string,
) *CleanUseCase {
	t.Helper()
	initI18nForTest(t)
	return NewCleanUseCase(&Dependencies{
		StateManager: &mocks.MockStateManager{
			LoadGlobalStateFunc: func() (*models.GlobalState, error) {
				return &models.GlobalState{Projects: projects}, nil
			},
			RemoveProjectFunc: func(name string) error {
				*removed = append(*removed, name)
				return nil
			},
		},
		DockerRunner: &mocks.MockDockerRunner{
			IsProjectActiveFunc: func(_ context.Context, ws, proj string) (bool, error) {
				return active(ws, proj)
			},
		},
	})
}

// The entry earns its place by having containers. One that does not is
// what `up` wrote and nothing ever removed.
func TestPruneStaleProjectStates_DropsInactive(t *testing.T) {
	var removed []string
	uc := cleanUCWithState(t, map[string]models.ProjectState{
		"gone": {Name: "gone", Workspace: "ws"},
	}, func(string, string) (bool, error) { return false, nil }, &removed)

	actions := uc.pruneStaleProjectStates(context.Background(), false)
	if len(removed) != 1 || removed[0] != "gone" {
		t.Errorf("removed = %v, want [gone]", removed)
	}
	if len(actions) != 1 || !strings.Contains(actions[0], "gone") {
		t.Errorf("actions = %v, want one naming the project", actions)
	}
}

func TestPruneStaleProjectStates_KeepsActive(t *testing.T) {
	var removed []string
	uc := cleanUCWithState(t, map[string]models.ProjectState{
		"alive": {Name: "alive", Workspace: "ws"},
	}, func(string, string) (bool, error) { return true, nil }, &removed)

	if actions := uc.pruneStaleProjectStates(context.Background(), false); len(actions) != 0 {
		t.Errorf("actions = %v, want none for a running project", actions)
	}
	if len(removed) != 0 {
		t.Errorf("removed %v, want nothing", removed)
	}
}

// "The daemon did not answer" and "the project is gone" must not lead to
// the same deletion.
func TestPruneStaleProjectStates_KeepsOnProbeFailure(t *testing.T) {
	var removed []string
	uc := cleanUCWithState(t, map[string]models.ProjectState{
		"unknown": {Name: "unknown", Workspace: "ws"},
	}, func(string, string) (bool, error) { return false, errors.New("docker unreachable") }, &removed)

	uc.pruneStaleProjectStates(context.Background(), false)
	if len(removed) != 0 {
		t.Errorf("removed %v on a failed probe, want nothing", removed)
	}
}

func TestPruneStaleProjectStates_DryRunWritesNothing(t *testing.T) {
	var removed []string
	uc := cleanUCWithState(t, map[string]models.ProjectState{
		"gone": {Name: "gone", Workspace: "ws"},
	}, func(string, string) (bool, error) { return false, nil }, &removed)

	actions := uc.pruneStaleProjectStates(context.Background(), true)
	if len(removed) != 0 {
		t.Errorf("dry run removed %v, want nothing", removed)
	}
	if len(actions) != 1 || !strings.Contains(actions[0], "gone") {
		t.Errorf("actions = %v, want the would-be action", actions)
	}
}
