package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"raioz/internal/domain/models"
	"raioz/internal/state"
)

// stubPortProbe replaces the probe hostServiceStatus consults and restores
// it when the test ends.
func stubPortProbe(t *testing.T, listening bool) {
	t.Helper()
	prev := hostStatusPortProbe
	hostStatusPortProbe = func(context.Context, int) bool { return listening }
	t.Cleanup(func() { hostStatusPortProbe = prev })
}

func TestHostServiceStatus(t *testing.T) {
	cases := []struct {
		name      string
		port      int
		listening bool
		want      string
	}{
		{"port declared and served", 3000, true, statusRunning},
		{"port declared, nobody listening", 3000, false, statusStarting},
		{"no port declared falls back to the PID", 0, false, statusRunning},
		{"negative port is not a declaration", -1, false, statusRunning},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stubPortProbe(t, tc.listening)
			if got := hostServiceStatus(t.Context(), tc.port); got != tc.want {
				t.Errorf("hostServiceStatus(port=%d, listening=%v) = %q, want %q",
					tc.port, tc.listening, got, tc.want)
			}
		})
	}
}

// The regression this whole change exists for: a live wrapper PID over a
// service that never bound its port used to print "running".
func TestStatusYAML_LivePIDClosedPortIsNotRunning(t *testing.T) {
	initI18nForTest(t)
	stubPortProbe(t, false)

	out := captureStdout(t, func() {
		if err := statusForLiveHostPID(t, 3000); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, statusStarting) {
		t.Errorf("status output = %q, want it to report %q", out, statusStarting)
	}
	if strings.Contains(out, statusRunning) {
		t.Errorf("status output = %q, must not report %q over a closed port", out, statusRunning)
	}
}

func TestStatusYAML_LivePIDServedPortIsRunning(t *testing.T) {
	initI18nForTest(t)
	stubPortProbe(t, true)

	out := captureStdout(t, func() {
		if err := statusForLiveHostPID(t, 3000); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, statusRunning) {
		t.Errorf("status output = %q, want it to report %q", out, statusRunning)
	}
}

// A service without `port:` has nothing better than its PID, so it must
// keep reporting running rather than regress to a permanent "starting".
func TestStatusYAML_NoPortKeepsPIDAnswer(t *testing.T) {
	initI18nForTest(t)
	stubPortProbe(t, false)

	out := captureStdout(t, func() {
		if err := statusForLiveHostPID(t, 0); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, statusRunning) {
		t.Errorf("status output = %q, want it to report %q", out, statusRunning)
	}
}

// statusForLiveHostPID runs StatusYAML over a one-service project whose
// recorded PID is this test process — alive by construction, which is
// exactly the wrapper-still-alive situation.
func statusForLiveHostPID(t *testing.T, port int) error {
	t.Helper()
	tmpDir := t.TempDir()
	ls := &models.LocalState{HostPIDs: map[string]int{"api": os.Getpid()}}
	if err := state.SaveLocalState(tmpDir, ls); err != nil {
		t.Fatalf("save state: %v", err)
	}

	uc := NewStatusUseCase(&Dependencies{})
	proj := &YAMLProject{
		ProjectName: "test",
		ConfigPath:  filepath.Join(tmpDir, "raioz.yaml"),
		Deps: &models.Deps{
			Project: models.Project{Name: "test"},
			Services: map[string]models.Service{
				"api": {Source: models.SourceConfig{Path: "."}, Port: port},
			},
			Infra: map[string]models.InfraEntry{},
		},
	}
	return uc.StatusYAML(context.Background(), proj, nil)
}
