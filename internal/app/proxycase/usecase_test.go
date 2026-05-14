package proxycase

import (
	"context"
	"errors"
	"testing"

	"raioz/internal/domain/models"
	"raioz/internal/mocks"
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

func TestRunPreflight_PublishGatesPortChecks(t *testing.T) {
	checks := RunPreflight(context.Background(), PreflightInput{Publish: false, TLSMode: "letsencrypt"})
	for _, c := range checks {
		if c.Name == "host port 80" || c.Name == "host port 443" {
			t.Errorf("port check %q should not run when publish=false", c.Name)
		}
	}
}

func TestRunPreflight_MkcertRequiredOnDefaultTLS(t *testing.T) {
	checks := RunPreflight(context.Background(), PreflightInput{Publish: false})
	var saw bool
	for _, c := range checks {
		if c.Name == "mkcert" {
			saw = true
			// Required iff TLS is mkcert (or empty = default mkcert).
			if !c.Required {
				t.Errorf("mkcert check should be Required=true with default TLS")
			}
		}
	}
	if !saw {
		t.Error("expected a mkcert check in the result set")
	}
}

func TestRunPreflight_PortChecksRunWhenPublishTrue(t *testing.T) {
	checks := RunPreflight(context.Background(), PreflightInput{Publish: true})
	names := map[string]bool{}
	for _, c := range checks {
		names[c.Name] = true
	}
	if !names["host port 80"] {
		t.Error("expected host port 80 check when publish=true")
	}
	if !names["host port 443"] {
		t.Error("expected host port 443 check when publish=true")
	}
}
