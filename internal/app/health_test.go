package app

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"raioz/internal/domain/models"
)

func TestNewHealthUseCase(t *testing.T) {
	deps := newFullMockDeps()
	uc := NewHealthUseCase(deps)
	if uc == nil {
		t.Fatal("expected non-nil HealthUseCase")
	}
}

// stubHealthProbe replaces the HTTP probe the use case consults.
func stubHealthProbe(t *testing.T, answer bool) {
	t.Helper()
	prev := healthEndpointProbe
	healthEndpointProbe = func(context.Context, string) bool { return answer }
	t.Cleanup(func() { healthEndpointProbe = prev })
}

func healthProject(t *testing.T, svcs map[string]models.Service, infra map[string]models.InfraEntry) *YAMLProject {
	t.Helper()
	if svcs == nil {
		svcs = map[string]models.Service{}
	}
	if infra == nil {
		infra = map[string]models.InfraEntry{}
	}
	return &YAMLProject{
		ProjectName: "test",
		ConfigPath:  filepath.Join(t.TempDir(), "raioz.yaml"),
		Deps: &models.Deps{
			Project:  models.Project{Name: "test"},
			Services: svcs,
			Infra:    infra,
		},
	}
}

// The regression this rewrite exists for: health reported "not running"
// over a project that was entirely up, because it read a field only
// .raioz.json could set.
func TestHealthUseCase_AnsweringEndpointIsHealthy(t *testing.T) {
	initI18nForTest(t)
	stubHealthProbe(t, true)
	stubContainerProbe(t, ContainerState{Status: "running"}, true)

	uc := NewHealthUseCase(&Dependencies{})
	proj := healthProject(t, map[string]models.Service{
		"api": {Port: 3000, HealthEndpoint: "/health"},
	}, nil)

	out := captureStdout(t, func() {
		if err := uc.reportHealth(context.Background(), proj); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, "healthy") {
		t.Errorf("output = %q, want a healthy verdict", out)
	}
}

// A non-zero exit is the point: a script has to be able to branch on it.
func TestHealthUseCase_SilentEndpointExitsNonZero(t *testing.T) {
	initI18nForTest(t)
	stubHealthProbe(t, false)

	uc := NewHealthUseCase(&Dependencies{})
	proj := healthProject(t, map[string]models.Service{
		"api": {Port: 3000, HealthEndpoint: "/health"},
	}, nil)

	var err error
	_ = captureStdout(t, func() { err = uc.reportHealth(context.Background(), proj) })
	if err == nil {
		t.Fatal("expected an unhealthy project to surface as an error")
	}
}

// A container in a restart loop reads "running" in most samples; the
// restart count is what makes it unhealthy. Probed through proxy.target,
// which is the service path that consults the container directly.
func TestHealthUseCase_LoopingContainerIsUnhealthy(t *testing.T) {
	initI18nForTest(t)
	stubContainerProbe(t, ContainerState{Status: "running", Restarts: 12}, true)

	uc := NewHealthUseCase(&Dependencies{})
	proj := healthProject(t, map[string]models.Service{
		"keycloak": {ProxyOverride: &models.ServiceProxyOverride{Target: "test-keycloak"}},
	}, nil)

	var err error
	out := captureStdout(t, func() { err = uc.reportHealth(context.Background(), proj) })
	if err == nil {
		t.Fatal("a looping container must not pass as healthy")
	}
	if !strings.Contains(out, "restarts:12") {
		t.Errorf("output = %q, want the restart count", out)
	}
}

// health: without port: cannot be probed; say so on the row instead of
// reporting a verdict the probe never made.
func TestHealthUseCase_EndpointWithoutPortSaysSo(t *testing.T) {
	initI18nForTest(t)
	stubHealthProbe(t, true)

	uc := NewHealthUseCase(&Dependencies{})
	proj := healthProject(t, map[string]models.Service{
		"api": {HealthEndpoint: "/health"},
	}, nil)

	var err error
	out := captureStdout(t, func() { err = uc.reportHealth(context.Background(), proj) })
	if err == nil {
		t.Fatal("an unprobeable service is not a healthy one")
	}
	if !strings.Contains(out, "port") {
		t.Errorf("output = %q, want the row to name the missing port:", out)
	}
}

func TestHealthUseCase_NothingDeclared(t *testing.T) {
	initI18nForTest(t)
	uc := NewHealthUseCase(&Dependencies{})

	var err error
	_ = captureStdout(t, func() { err = uc.reportHealth(context.Background(), healthProject(t, nil, nil)) })
	if err != nil {
		t.Fatalf("an empty project is not unhealthy: %v", err)
	}
}
