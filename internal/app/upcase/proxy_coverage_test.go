package upcase

import (
	"context"
	"errors"
	"testing"

	"raioz/internal/config"
	"raioz/internal/detect"
	"raioz/internal/domain/interfaces"
	"raioz/internal/mocks"
)

func TestStartProxy_AddRoutesAndStart(t *testing.T) {
	pm := &mocks.MockProxyManager{}
	uc := &UseCase{
		deps: &Dependencies{
			ProxyManager: pm,
		},
	}

	deps := &config.Deps{
		Project:  config.Project{Name: "myproj"},
		Proxy:    true,
		Services: map[string]config.Service{},
		Infra:    map[string]config.InfraEntry{},
	}
	detections := DetectionMap{
		"api": {Runtime: detect.RuntimeGo, Port: 8080},
		"web": {Runtime: detect.RuntimeNPM, Port: 3000},
	}
	serviceNames := []string{"api", "web"}

	uc.startProxy(context.Background(), deps, detections, serviceNames, "test-net")

	if pm.ProjectName != "myproj" {
		t.Errorf("ProjectName = %q, want myproj", pm.ProjectName)
	}
	if !pm.StartCalled {
		t.Error("Start was not called")
	}
	if len(pm.AddedRoutes) != 2 {
		t.Errorf("expected 2 routes, got %d", len(pm.AddedRoutes))
	}
}

func TestStartProxy_SetsDomainAndTLS(t *testing.T) {
	pm := &mocks.MockProxyManager{}
	uc := &UseCase{
		deps: &Dependencies{
			ProxyManager: pm,
		},
	}

	deps := &config.Deps{
		Project: config.Project{Name: "proj"},
		Proxy:   true,
		ProxyConfig: &config.ProxyConfig{
			Domain: "acme.localhost",
			TLS:    "mkcert",
		},
		Services: map[string]config.Service{},
		Infra:    map[string]config.InfraEntry{},
	}
	detections := DetectionMap{}

	uc.startProxy(context.Background(), deps, detections, nil, "net")

	if pm.Domain != "acme.localhost" {
		t.Errorf("Domain = %q, want acme.localhost", pm.Domain)
	}
	if pm.TLSMode != "mkcert" {
		t.Errorf("TLSMode = %q, want mkcert", pm.TLSMode)
	}
}

func TestStartProxy_StartFailure(t *testing.T) {
	pm := &mocks.MockProxyManager{
		StartFunc: func(ctx context.Context, networkName string) error {
			return errors.New("proxy start failed")
		},
	}
	uc := &UseCase{
		deps: &Dependencies{
			ProxyManager: pm,
		},
	}

	deps := &config.Deps{
		Project:  config.Project{Name: "proj"},
		Proxy:    true,
		Services: map[string]config.Service{},
		Infra:    map[string]config.InfraEntry{},
	}
	detections := DetectionMap{
		"api": {Runtime: detect.RuntimeGo, Port: 8080},
	}

	// Proxy failure must bubble up — `up` aborts hard when `proxy: true` is
	// declared but the proxy can't start and stdin isn't a tty.
	err := uc.startProxy(context.Background(), deps, detections, []string{"api"}, "net")
	if err == nil {
		t.Fatal("expected startProxy to return error when ProxyManager.Start fails")
	}
	if !pm.StartCalled {
		t.Error("Start should have been called even if it fails")
	}
}

// withInteractiveProxy temporarily forces the interactive branch + injects a
// deterministic prompt response, restoring originals on cleanup.
func withInteractiveProxy(t *testing.T, action int) {
	t.Helper()
	prevTTY, prevPrompt := stdinIsInteractiveFn, proxyFailurePrompt
	stdinIsInteractiveFn = func() bool { return true }
	proxyFailurePrompt = func() int { return action }
	t.Cleanup(func() {
		stdinIsInteractiveFn = prevTTY
		proxyFailurePrompt = prevPrompt
	})
}

func TestStartProxy_InteractiveSkip(t *testing.T) {
	withInteractiveProxy(t, proxyActionSkip)

	pm := &mocks.MockProxyManager{
		StartFunc: func(context.Context, string) error {
			return errors.New("ports busy")
		},
	}
	uc := &UseCase{deps: &Dependencies{ProxyManager: pm}}
	deps := &config.Deps{
		Project:  config.Project{Name: "proj"},
		Proxy:    true,
		Services: map[string]config.Service{},
		Infra:    map[string]config.InfraEntry{},
	}

	err := uc.startProxy(context.Background(), deps, DetectionMap{}, nil, "net")
	if err != nil {
		t.Errorf("skip must not bubble an error, got %v", err)
	}
}

func TestStartProxy_InteractiveRetrySucceeds(t *testing.T) {
	withInteractiveProxy(t, proxyActionRetry)

	calls := 0
	pm := &mocks.MockProxyManager{
		StartFunc: func(context.Context, string) error {
			calls++
			if calls == 1 {
				return errors.New("ports busy")
			}
			return nil
		},
	}
	uc := &UseCase{deps: &Dependencies{ProxyManager: pm}}
	deps := &config.Deps{
		Project:  config.Project{Name: "proj"},
		Proxy:    true,
		Services: map[string]config.Service{},
		Infra:    map[string]config.InfraEntry{},
	}

	err := uc.startProxy(context.Background(), deps, DetectionMap{}, nil, "net")
	if err != nil {
		t.Errorf("retry success must not bubble, got %v", err)
	}
	if calls != 2 {
		t.Errorf("expected 2 Start calls (initial + retry), got %d", calls)
	}
}

func TestStartProxy_InteractiveCancel(t *testing.T) {
	withInteractiveProxy(t, proxyActionCancel)

	pm := &mocks.MockProxyManager{
		StartFunc: func(context.Context, string) error {
			return errors.New("ports busy")
		},
	}
	uc := &UseCase{deps: &Dependencies{ProxyManager: pm}}
	deps := &config.Deps{
		Project:  config.Project{Name: "proj"},
		Proxy:    true,
		Services: map[string]config.Service{},
		Infra:    map[string]config.InfraEntry{},
	}

	err := uc.startProxy(context.Background(), deps, DetectionMap{}, nil, "net")
	if err == nil {
		t.Error("cancel must propagate the original error")
	}
}

func TestStartProxy_AddRouteError(t *testing.T) {
	pm := &mocks.MockProxyManager{}
	pm.AddRouteFunc = func(ctx context.Context, route interfaces.ProxyRoute) error {
		return errors.New("route add failed")
	}

	uc := &UseCase{
		deps: &Dependencies{
			ProxyManager: pm,
		},
	}

	deps := &config.Deps{
		Project:  config.Project{Name: "proj"},
		Services: map[string]config.Service{},
		Infra:    map[string]config.InfraEntry{},
	}
	detections := DetectionMap{
		"api": {Runtime: detect.RuntimeGo, Port: 8080},
	}

	// Should not panic — route add failure is logged as warning
	uc.startProxy(context.Background(), deps, detections, []string{"api"}, "net")
}

func TestStartProxy_NoProxyConfig(t *testing.T) {
	pm := &mocks.MockProxyManager{}
	uc := &UseCase{
		deps: &Dependencies{
			ProxyManager: pm,
		},
	}

	deps := &config.Deps{
		Project:     config.Project{Name: "proj"},
		Proxy:       true,
		ProxyConfig: nil, // no custom config
		Services:    map[string]config.Service{},
		Infra:       map[string]config.InfraEntry{},
	}
	detections := DetectionMap{}

	uc.startProxy(context.Background(), deps, detections, nil, "net")

	if pm.Domain != "" {
		t.Errorf("Domain should be empty with nil ProxyConfig, got %q", pm.Domain)
	}
}
