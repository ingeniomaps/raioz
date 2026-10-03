package upcase

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"raioz/internal/domain/interfaces"
	"raioz/internal/domain/models"
)

// A repeated up finds the project's own host service on its port. That is
// the service being up, not a conflict: no prompt, and the run records the
// PID so the start step adopts it.
func TestResolvePortBindConflicts_OwnHostService(t *testing.T) {
	initI18nUp(t)
	withStubbedTTY(t, false)

	prev := ownHostServicePIDFn
	ownHostServicePIDFn = func(c PortBindConflict, _ string) int {
		if c.Name == "web" {
			return 4242
		}
		return 0
	}
	t.Cleanup(func() { ownHostServicePIDFn = prev })

	deps := &models.Deps{Project: models.Project{Name: "bencha"}}

	t.Run("own service is adopted", func(t *testing.T) {
		result := &PortAllocResult{}
		err := resolvePortBindConflicts(context.Background(),
			[]PortBindConflict{{Kind: "service", Name: "web", Port: 38101}},
			result, "raioz.yaml", deps, "")
		if err != nil {
			t.Fatalf("own host service reported as a conflict: %v", err)
		}
		if result.RunningHost["web"] != 4242 {
			t.Errorf("RunningHost = %v, want web:4242", result.RunningHost)
		}
	})

	t.Run("foreign occupant still fails", func(t *testing.T) {
		result := &PortAllocResult{}
		err := resolvePortBindConflicts(context.Background(),
			[]PortBindConflict{{Kind: "service", Name: "api", Port: 38102}},
			result, "raioz.yaml", deps, "")
		if err == nil {
			t.Fatal("expected a port conflict for a port raioz does not own")
		}
		if len(result.RunningHost) != 0 {
			t.Errorf("RunningHost = %v, want empty", result.RunningHost)
		}
	})
}

type recordingDispatcher struct {
	started []string
	adopted map[string]int
}

func (d *recordingDispatcher) Start(_ context.Context, svc interfaces.ServiceContext) error {
	d.started = append(d.started, svc.Name)
	return nil
}

func (d *recordingDispatcher) GetHostPID(name string) int { return d.adopted[name] }

func (d *recordingDispatcher) AdoptHostPID(name string, pid int) {
	if d.adopted == nil {
		d.adopted = map[string]int{}
	}
	d.adopted[name] = pid
}

func TestStartServices_AdoptsRunningHostService(t *testing.T) {
	initI18nUp(t)

	deps := &models.Deps{
		Project: models.Project{Name: "bencha"},
		Services: map[string]models.Service{
			"web": {Source: models.SourceConfig{Kind: "local", Path: t.TempDir()}},
			"api": {Source: models.SourceConfig{Kind: "local", Path: t.TempDir()}},
		},
	}
	detections := DetectionMap{
		"web": {Runtime: models.RuntimeNPM, DevCommand: "npm run dev"},
		"api": {Runtime: models.RuntimeGo, DevCommand: "go run ."},
	}
	dispatcher := &recordingDispatcher{}
	uc := &UseCase{deps: &Dependencies{}}

	err := uc.startServices(context.Background(), startServicesParams{
		deps:         deps,
		detections:   detections,
		serviceNames: []string{"web", "api"},
		portAllocs:   &PortAllocResult{RunningHost: map[string]int{"web": 4242}},
		dispatcher:   dispatcher,
		projectDir:   t.TempDir(),
	})
	if err != nil {
		t.Fatalf("startServices: %v", err)
	}
	if len(dispatcher.started) != 1 || dispatcher.started[0] != "api" {
		t.Errorf("started = %v, want only api", dispatcher.started)
	}
	if dispatcher.adopted["web"] != 4242 {
		t.Errorf("adopted = %v, want web:4242", dispatcher.adopted)
	}
}

// Two services without `port:` both default to 3000; the first up gave
// them 3000 and 3001. A repeated up finds both ports busy with the
// project's own processes: each must keep its port, not be bumped to the
// next free one and started a second time.
func TestAllocateHostPortsOwned_KeepsHeldPorts(t *testing.T) {
	prev := portInUseProbe
	portInUseProbe = func(port string) (bool, error) {
		return port == "3000" || port == "3001" || port == "3002", nil
	}
	t.Cleanup(func() { portInUseProbe = prev })

	deps := &models.Deps{
		Project: models.Project{Name: "rzc"},
		Services: map[string]models.Service{
			"a": {Source: models.SourceConfig{Kind: "local", Path: "a"}},
			"b": {Source: models.SourceConfig{Kind: "local", Path: "b"}},
		},
	}
	detections := DetectionMap{
		"a": {Runtime: models.RuntimeNPM, Port: 3000},
		"b": {Runtime: models.RuntimeNPM, Port: 3000},
	}

	tests := []struct {
		name  string
		owned hostPortOwner
		want  map[string]int
	}{
		{
			name:  "nothing running bumps past the busy ports",
			owned: nil,
			want:  map[string]int{"a": 3003, "b": 3004},
		},
		{
			name: "running services keep their ports",
			owned: func(svc string) []int {
				return map[string][]int{"a": {3000}, "b": {3001}}[svc]
			},
			want: map[string]int{"a": 3000, "b": 3001},
		},
		{
			name: "only one running: the other is allocated around it",
			owned: func(svc string) []int {
				return map[string][]int{"b": {3001}}[svc]
			},
			want: map[string]int{"a": 3003, "b": 3001},
		},
		{
			name: "ports below the wanted one are not the service port",
			owned: func(svc string) []int {
				return map[string][]int{"a": {2999, 3002, 9229}}[svc]
			},
			want: map[string]int{"a": 3002, "b": 3003},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := AllocateHostPortsOwned(deps, detections, tc.owned)
			if err != nil {
				t.Fatalf("AllocateHostPortsOwned: %v", err)
			}
			for svc, want := range tc.want {
				if got.Services[svc].Port != want {
					t.Errorf("%s = %d, want %d", svc, got.Services[svc].Port, want)
				}
			}
		})
	}
}

// A compose / Dockerfile service declares the port its container listens
// on with `port:`. The proxy route was built without it and dialed :80.
func TestBuildProxyRoute_DockerServiceUsesDeclaredPort(t *testing.T) {
	dir := t.TempDir()
	deps := &models.Deps{
		Project: models.Project{Name: "rzb1"},
		Services: map[string]models.Service{
			"api": {Source: models.SourceConfig{Kind: "local", Path: dir}, Port: 3000},
		},
	}
	det := models.DetectResult{Runtime: models.RuntimeDockerfile}

	route := buildProxyRoute(context.Background(), nil, deps, "api", &det)
	if route.Port != 3000 {
		t.Errorf("route port = %d, want 3000", route.Port)
	}
}

func TestWaitForServiceEndpoints_ContainerAddress(t *testing.T) {
	initI18nUp(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte("FROM alpine\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	deps := &models.Deps{
		Project: models.Project{Name: "rzb1"},
		Services: map[string]models.Service{
			"api": {
				Source: models.SourceConfig{Kind: "local", Path: dir},
				Port:   3000, HealthEndpoint: "/health",
			},
		},
	}

	prevIP := serviceContainerIPFn
	serviceContainerIPFn = func(context.Context, string, string) string { return "10.213.0.8" }
	t.Cleanup(func() { serviceContainerIPFn = prevIP })

	var answered string
	prevProbe := endpointProbe
	endpointProbe = func(_ context.Context, url string) bool {
		if strings.Contains(url, "10.213.0.8") {
			answered = url
			return true
		}
		return false
	}
	t.Cleanup(func() { endpointProbe = prevProbe })

	waitForServiceEndpoints(context.Background(), deps, []string{"api"})
	if answered != "http://10.213.0.8:3000/health" {
		t.Errorf("container address never probed, got %q", answered)
	}
}
