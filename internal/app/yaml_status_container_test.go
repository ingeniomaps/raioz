package app

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"raioz/internal/domain/models"
)

// stubContainerProbe replaces the probe the status paths consult and
// restores it when the test ends.
func stubContainerProbe(t *testing.T, st ContainerState, found bool) {
	t.Helper()
	prev := dockerStateProbe
	dockerStateProbe = func(context.Context, string) (ContainerState, bool) {
		return st, found
	}
	t.Cleanup(func() { dockerStateProbe = prev })
}

func TestParseContainerState(t *testing.T) {
	cases := []struct {
		name         string
		out          string
		wantFound    bool
		wantStatus   string
		wantRestarts int
	}{
		{"healthy", "running 0\n", true, "running", 0},
		{"crash loop", "running 8170\n", true, "running", 8170},
		{"backoff phase", "restarting 8170\n", true, "restarting", 8170},
		{"count missing", "running\n", true, "running", 0},
		{"count not a number", "running x\n", true, "running", 0},
		{"empty output", "\n", false, "", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, found := parseContainerState(tc.out)
			if found != tc.wantFound {
				t.Fatalf("parseContainerState(%q) found = %v, want %v", tc.out, found, tc.wantFound)
			}
			if got.Status != tc.wantStatus || got.Restarts != tc.wantRestarts {
				t.Errorf("parseContainerState(%q) = %+v, want {%q %d}",
					tc.out, got, tc.wantStatus, tc.wantRestarts)
			}
		})
	}
}

func TestFormatContainerStatus(t *testing.T) {
	cases := []struct {
		name string
		st   ContainerState
		want string
	}{
		{"healthy container stays a bare status", ContainerState{Status: "running"}, "running"},
		{"crash loop caught mid-boot", ContainerState{Status: "running", Restarts: 8170},
			"running restarts:8170"},
		{"crash loop caught in backoff", ContainerState{Status: "restarting", Restarts: 8170},
			"restarting restarts:8170"},
		{"loop that gave up", ContainerState{Status: "exited", Restarts: 6}, "exited restarts:6"},
		{"paused is not stopped", ContainerState{Status: "paused"}, "paused"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := formatContainerStatus(tc.st); got != tc.want {
				t.Errorf("formatContainerStatus(%+v) = %q, want %q", tc.st, got, tc.want)
			}
		})
	}
}

// The regression this change exists for. A container in a restart loop
// spends most of its cycle in State.Status "running", so the status alone
// reads exactly like a healthy service — the restart count is what has to
// reach the table.
func TestStatusYAML_CrashLoopIsVisible(t *testing.T) {
	initI18nForTest(t)
	stubContainerProbe(t, ContainerState{Status: "running", Restarts: 8170}, true)

	out := captureStdout(t, func() {
		if err := statusForProxyTarget(t); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, "restarts:8170") {
		t.Errorf("status output = %q, want it to expose the restart count", out)
	}
}

// A container that Docker reports as restarting must not be flattened into
// "stopped": the loop is the whole signal.
func TestStatusYAML_RestartingIsReportedVerbatim(t *testing.T) {
	initI18nForTest(t)
	stubContainerProbe(t, ContainerState{Status: "restarting", Restarts: 3}, true)

	out := captureStdout(t, func() {
		if err := statusForProxyTarget(t); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, "restarting") {
		t.Errorf("status output = %q, want it to report %q", out, "restarting")
	}
	if strings.Contains(out, statusStopped) {
		t.Errorf("status output = %q, must not flatten a restarting container into %q",
			out, statusStopped)
	}
}

// A healthy container keeps the plain reading it always had.
func TestStatusYAML_HealthyContainerStaysRunning(t *testing.T) {
	initI18nForTest(t)
	stubContainerProbe(t, ContainerState{Status: "running"}, true)

	out := captureStdout(t, func() {
		if err := statusForProxyTarget(t); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, statusRunning) {
		t.Errorf("status output = %q, want it to report %q", out, statusRunning)
	}
	if strings.Contains(out, "restarts:") {
		t.Errorf("status output = %q, must not carry a restart marker at zero restarts", out)
	}
}

// A missing container falls through to the PID branch, which has no PID
// recorded here — the service is stopped.
func TestStatusYAML_MissingTargetFallsThrough(t *testing.T) {
	initI18nForTest(t)
	stubContainerProbe(t, ContainerState{}, false)

	out := captureStdout(t, func() {
		if err := statusForProxyTarget(t); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, statusStopped) {
		t.Errorf("status output = %q, want it to report %q", out, statusStopped)
	}
}

func TestYAMLProject_ContainerState_NoContainer(t *testing.T) {
	stubContainerProbe(t, ContainerState{Status: "running", Restarts: 9}, true)

	// No Infra entry and a project name that resolves to nothing: the
	// probe must never be consulted.
	p := &YAMLProject{ProjectName: "zzz-test-nonexistent-proj"}
	if got := p.ContainerState(context.Background(), "noservice"); got.Status != statusStopped {
		t.Errorf("ContainerState of a nonexistent container = %+v, want %q", got, statusStopped)
	}
}

// statusForProxyTarget runs StatusYAML over a one-service project that
// declares `proxy.target:`, which is the branch where the container state
// is the source of truth.
func statusForProxyTarget(t *testing.T) error {
	t.Helper()
	tmpDir := t.TempDir()

	uc := NewStatusUseCase(&Dependencies{})
	proj := &YAMLProject{
		ProjectName: "test",
		ConfigPath:  filepath.Join(tmpDir, "raioz.yaml"),
		Deps: &models.Deps{
			Project: models.Project{Name: "test"},
			Services: map[string]models.Service{
				"keycloak": {
					Source: models.SourceConfig{Path: ".", Command: "make start"},
					ProxyOverride: &models.ServiceProxyOverride{
						Target: "test-keycloak",
						Port:   8080,
					},
				},
			},
			Infra: map[string]models.InfraEntry{},
		},
	}
	return uc.StatusYAML(context.Background(), proj, nil)
}
