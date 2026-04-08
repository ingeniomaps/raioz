package app

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"raioz/internal/i18n"
	"raioz/internal/mocks"
	"raioz/internal/workspace"
)

func initI18nWs(t *testing.T) {
	t.Helper()
	os.Setenv("RAIOZ_LANG", "en")
	t.Cleanup(func() { os.Unsetenv("RAIOZ_LANG") })
	i18n.Init("en")
}

func TestNewWorkspaceUseCase(t *testing.T) {
	uc := NewWorkspaceUseCase(&Dependencies{})
	if uc == nil {
		t.Fatal("should return non-nil")
	}
}

func TestWorkspaceCurrent(t *testing.T) {
	initI18nWs(t)

	t.Run("with active workspace", func(t *testing.T) {
		uc := NewWorkspaceUseCase(&Dependencies{
			Workspace: &mocks.MockWorkspaceManager{
				GetActiveWorkspaceFunc: func() (string, error) {
					return "billing", nil
				},
			},
		})
		var buf bytes.Buffer
		uc.Out = &buf

		err := uc.Current()
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		if !strings.Contains(buf.String(), "billing") {
			t.Errorf("should show active workspace\ngot: %s", buf.String())
		}
	})

	t.Run("no active workspace", func(t *testing.T) {
		uc := NewWorkspaceUseCase(&Dependencies{
			Workspace: &mocks.MockWorkspaceManager{
				GetActiveWorkspaceFunc: func() (string, error) {
					return "", nil
				},
			},
		})
		var buf bytes.Buffer
		uc.Out = &buf

		err := uc.Current()
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		if !strings.Contains(strings.ToLower(buf.String()), "no active") {
			t.Errorf("should show no active hint\ngot: %s", buf.String())
		}
	})
}

func TestWorkspaceUseEmptyName(t *testing.T) {
	initI18nWs(t)
	uc := NewWorkspaceUseCase(&Dependencies{})
	err := uc.Use("")
	if err == nil {
		t.Error("expected error for empty name")
	}
}

func TestWorkspaceUseNewWorkspace(t *testing.T) {
	initI18nWs(t)

	ws := &workspace.Workspace{Root: "/tmp/test"}
	uc := NewWorkspaceUseCase(&Dependencies{
		Workspace: &mocks.MockWorkspaceManager{
			WorkspaceExistsFunc: func(name string) (bool, error) { return false, nil },
			ResolveFunc:         func(name string) (*workspace.Workspace, error) { return ws, nil },
			GetActiveWorkspaceFunc: func() (string, error) { return "", nil },
			SetActiveWorkspaceFunc: func(name string) error { return nil },
		},
	})
	var buf bytes.Buffer
	uc.Out = &buf

	err := uc.Use("new-ws")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "new-ws") {
		t.Errorf("should mention workspace name\ngot: %s", output)
	}
	if !strings.Contains(output, "Created") && !strings.Contains(output, "created") {
		t.Errorf("should say created for new workspace\ngot: %s", output)
	}
}

func TestWorkspaceUseExistingWorkspace(t *testing.T) {
	initI18nWs(t)

	ws := &workspace.Workspace{Root: "/tmp/test"}
	uc := NewWorkspaceUseCase(&Dependencies{
		Workspace: &mocks.MockWorkspaceManager{
			WorkspaceExistsFunc: func(name string) (bool, error) { return true, nil },
			ResolveFunc:         func(name string) (*workspace.Workspace, error) { return ws, nil },
			GetActiveWorkspaceFunc: func() (string, error) { return "old", nil },
			SetActiveWorkspaceFunc: func(name string) error { return nil },
		},
	})
	var buf bytes.Buffer
	uc.Out = &buf

	err := uc.Use("existing-ws")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "already exists") {
		t.Errorf("should say already exists\ngot: %s", output)
	}
}

func TestWorkspaceDeleteExisting(t *testing.T) {
	initI18nWs(t)

	deleted := false
	uc := NewWorkspaceUseCase(&Dependencies{
		Workspace: &mocks.MockWorkspaceManager{
			WorkspaceExistsFunc: func(name string) (bool, error) { return true, nil },
			DeleteWorkspaceFunc: func(name string) error { deleted = true; return nil },
			GetActiveWorkspaceFunc: func() (string, error) { return "other", nil },
		},
	})
	var buf bytes.Buffer
	uc.Out = &buf

	err := uc.Delete("old-ws")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !deleted {
		t.Error("DeleteWorkspace should have been called")
	}
	if !strings.Contains(buf.String(), "old-ws") {
		t.Errorf("should mention deleted workspace\ngot: %s", buf.String())
	}
}

func TestWorkspaceDeleteNonexistent(t *testing.T) {
	initI18nWs(t)

	uc := NewWorkspaceUseCase(&Dependencies{
		Workspace: &mocks.MockWorkspaceManager{
			WorkspaceExistsFunc: func(name string) (bool, error) { return false, nil },
		},
	})
	var buf bytes.Buffer
	uc.Out = &buf

	err := uc.Delete("ghost")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !strings.Contains(buf.String(), "ghost") {
		t.Errorf("should mention workspace not found\ngot: %s", buf.String())
	}
}

func TestWorkspaceDeleteDeactivates(t *testing.T) {
	initI18nWs(t)

	deactivated := false
	uc := NewWorkspaceUseCase(&Dependencies{
		Workspace: &mocks.MockWorkspaceManager{
			WorkspaceExistsFunc:    func(name string) (bool, error) { return true, nil },
			DeleteWorkspaceFunc:    func(name string) error { return nil },
			GetActiveWorkspaceFunc: func() (string, error) { return "active-ws", nil },
			SetActiveWorkspaceFunc: func(name string) error { deactivated = true; return nil },
		},
	})
	var buf bytes.Buffer
	uc.Out = &buf

	err := uc.Delete("active-ws")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !deactivated {
		t.Error("should deactivate when deleting active workspace")
	}
	output := buf.String()
	if !strings.Contains(strings.ToLower(output), "deactivat") {
		t.Errorf("should mention deactivation\ngot: %s", output)
	}
}

func TestWorkspaceDeleteEmptyName(t *testing.T) {
	initI18nWs(t)
	uc := NewWorkspaceUseCase(&Dependencies{})
	err := uc.Delete("")
	if err == nil {
		t.Error("expected error for empty name")
	}
}

func TestWorkspaceListEmpty(t *testing.T) {
	initI18nWs(t)

	uc := NewWorkspaceUseCase(&Dependencies{
		Workspace: &mocks.MockWorkspaceManager{
			GetActiveWorkspaceFunc: func() (string, error) { return "", nil },
			ListWorkspacesFunc:     func() ([]string, error) { return []string{}, nil },
		},
	})
	var buf bytes.Buffer
	uc.Out = &buf

	err := uc.List()
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !strings.Contains(strings.ToLower(buf.String()), "no workspaces") {
		t.Errorf("should show no workspaces\ngot: %s", buf.String())
	}
}

func TestWorkspaceListWithActive(t *testing.T) {
	initI18nWs(t)

	uc := NewWorkspaceUseCase(&Dependencies{
		Workspace: &mocks.MockWorkspaceManager{
			GetActiveWorkspaceFunc: func() (string, error) { return "billing", nil },
			ListWorkspacesFunc:     func() ([]string, error) { return []string{"billing", "payments"}, nil },
		},
	})
	var buf bytes.Buffer
	uc.Out = &buf

	err := uc.List()
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "* billing") {
		t.Errorf("active should be marked\ngot: %s", output)
	}
	if !strings.Contains(output, "payments") {
		t.Error("should list all workspaces")
	}
}

func TestWorkspaceListNoActive(t *testing.T) {
	initI18nWs(t)

	uc := NewWorkspaceUseCase(&Dependencies{
		Workspace: &mocks.MockWorkspaceManager{
			GetActiveWorkspaceFunc: func() (string, error) { return "", nil },
			ListWorkspacesFunc:     func() ([]string, error) { return []string{"a", "b"}, nil },
		},
	})
	var buf bytes.Buffer
	uc.Out = &buf

	err := uc.List()
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !strings.Contains(strings.ToLower(buf.String()), "workspace use") {
		t.Errorf("should show set hint\ngot: %s", buf.String())
	}
}
