package app

import (
	"context"
	"fmt"
	"testing"

	"raioz/internal/domain/interfaces"
	"raioz/internal/workspace"
)

func TestPortsUseCase_Execute_WithProjectName(t *testing.T) {
	initI18nForTest(t)
	deps, wsMgr, dockerRunner := newTestDepsForPorts(t)
	// Resolve returns a workspace for the given project name
	wsMgr.ResolveFunc = func(projectName string) (*workspace.Workspace, error) {
		return &workspace.Workspace{Root: "/tmp/test"}, nil
	}
	wsMgr.GetBaseDirFromWorkspaceFunc = func(ws *workspace.Workspace) string {
		return "/tmp/base"
	}
	dockerRunner.GetAllActivePortsFunc = func(baseDir string) ([]interfaces.PortInfo, error) {
		if baseDir != "/tmp/base" {
			t.Errorf("expected /tmp/base, got %q", baseDir)
		}
		return []interfaces.PortInfo{}, nil
	}

	uc := NewPortsUseCase(deps)
	err := uc.Execute(context.Background(), PortsOptions{ProjectName: "my-proj"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPortsUseCase_Execute_GetBaseDirError(t *testing.T) {
	initI18nForTest(t)
	deps, wsMgr, _ := newTestDepsForPorts(t)
	wsMgr.GetBaseDirFunc = func() (string, error) {
		return "", fmt.Errorf("no base dir")
	}
	uc := NewPortsUseCase(deps)
	err := uc.Execute(context.Background(), PortsOptions{})
	if err == nil {
		t.Fatal("expected error for base dir failure, got nil")
	}
}

func TestPortsUseCase_Execute_GetPortsError(t *testing.T) {
	initI18nForTest(t)
	deps, _, dockerRunner := newTestDepsForPorts(t)
	dockerRunner.GetAllActivePortsFunc = func(baseDir string) ([]interfaces.PortInfo, error) {
		return nil, fmt.Errorf("docker error")
	}
	uc := NewPortsUseCase(deps)
	err := uc.Execute(context.Background(), PortsOptions{})
	if err == nil {
		t.Fatal("expected error for get ports failure, got nil")
	}
}
