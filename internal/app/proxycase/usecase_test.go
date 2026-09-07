package proxycase

import (
	"context"
	"errors"
	"testing"

	"raioz/internal/domain/models"
	"raioz/internal/mocks"
	"raioz/internal/naming"
)

func TestStatus_NotConfigured(t *testing.T) {
	uc := StatusUseCase{Deps: &Dependencies{}}
	_, err := uc.Execute(context.Background(), StatusOptions{})
	if !errors.Is(err, ErrProxyNotConfigured) {
		t.Fatalf("expected ErrProxyNotConfigured, got %v", err)
	}
}

func TestStatus_DelegatesToManager(t *testing.T) {
	mgr := &mocks.MockProxyManager{
		StatusFunc: func(ctx context.Context) (bool, error) {
			return true, nil
		},
	}
	uc := StatusUseCase{Deps: &Dependencies{
		ConfigLoader: &mocks.MockConfigLoader{},
		ProxyManager: mgr,
	}}
	running, err := uc.Execute(context.Background(), StatusOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !running {
		t.Error("expected running=true")
	}
}

func TestStatus_AppliesProjectScopeFromConfig(t *testing.T) {
	prev := naming.GetPrefix()
	t.Cleanup(func() { naming.SetPrefix(prev) })

	mgr := &mocks.MockProxyManager{}
	loader := &mocks.MockConfigLoader{
		LoadDepsFunc: func(_ string) (*models.Deps, []string, error) {
			return &models.Deps{
				Project:   models.Project{Name: "proj"},
				Workspace: "ws",
			}, nil, nil
		},
	}
	uc := StatusUseCase{Deps: &Dependencies{
		ConfigLoader: loader,
		ProxyManager: mgr,
	}}
	if _, err := uc.Execute(context.Background(), StatusOptions{ConfigPath: "/x/raioz.yaml"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mgr.ProjectName != "proj" {
		t.Errorf("expected ProjectName=proj, got %q", mgr.ProjectName)
	}
	if mgr.Workspace != "ws" {
		t.Errorf("expected Workspace=ws, got %q", mgr.Workspace)
	}
}

// applyScope must activate the workspace naming prefix so the manager's
// container-name helpers target <workspace>-proxy, not a default-prefixed
// name. Without it, stop/status silently no-op on the real shared proxy.
func TestStop_ActivatesWorkspacePrefix(t *testing.T) {
	prev := naming.GetPrefix()
	t.Cleanup(func() { naming.SetPrefix(prev) })
	naming.SetPrefix("") // ensure we start from the default prefix

	mgr := &mocks.MockProxyManager{}
	loader := &mocks.MockConfigLoader{
		LoadDepsFunc: func(_ string) (*models.Deps, []string, error) {
			return &models.Deps{
				Project:   models.Project{Name: "proj"},
				Workspace: "acme",
			}, nil, nil
		},
	}
	uc := StopUseCase{Deps: &Dependencies{ConfigLoader: loader, ProxyManager: mgr}}
	if err := uc.Execute(context.Background(), StopOptions{ConfigPath: "/x/raioz.yaml"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := naming.GetPrefix(); got != "acme" {
		t.Errorf("naming prefix = %q, want acme", got)
	}
	if mgr.Workspace != "acme" {
		t.Errorf("Workspace = %q, want acme", mgr.Workspace)
	}
}

func TestStop_NotConfigured(t *testing.T) {
	uc := StopUseCase{Deps: &Dependencies{}}
	err := uc.Execute(context.Background(), StopOptions{})
	if !errors.Is(err, ErrProxyNotConfigured) {
		t.Fatalf("expected ErrProxyNotConfigured, got %v", err)
	}
}

func TestStop_DelegatesToManager(t *testing.T) {
	stopped := false
	mgr := &mocks.MockProxyManager{
		StopFunc: func(ctx context.Context) error {
			stopped = true
			return nil
		},
	}
	uc := StopUseCase{Deps: &Dependencies{
		ConfigLoader: &mocks.MockConfigLoader{},
		ProxyManager: mgr,
	}}
	if err := uc.Execute(context.Background(), StopOptions{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !stopped {
		t.Error("expected ProxyManager.Stop to be invoked")
	}
}
