package host

import (
	"testing"
	"time"
)

func stubEnv(t *testing.T, vals map[string]string) {
	t.Helper()
	prev := osGetenv
	t.Cleanup(func() { osGetenv = prev })
	osGetenv = func(name string) string {
		return vals[name]
	}
}

func TestLauncherWaitTimeout(t *testing.T) {
	cases := []struct {
		name   string
		envVal string
		want   time.Duration
	}{
		{"default when unset", "", 60 * time.Second},
		{"override 90s", "90s", 90 * time.Second},
		{"override 2m", "2m", 2 * time.Minute},
		{"explicit opt-out", "0s", 0},
		{"unparseable falls back to default", "lots", 60 * time.Second},
		{"negative falls back to default", "-5s", 60 * time.Second},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stubEnv(t, map[string]string{launcherWaitTimeoutEnv: tc.envVal})
			if got := LauncherWaitTimeout(); got != tc.want {
				t.Errorf("LauncherWaitTimeout() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestLauncherDrainTimeout(t *testing.T) {
	cases := []struct {
		name   string
		envVal string
		want   time.Duration
	}{
		{"default when unset", "", 30 * time.Second},
		{"override 45s", "45s", 45 * time.Second},
		{"explicit opt-out", "0s", 0},
		{"unparseable falls back to default", "nope", 30 * time.Second},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stubEnv(t, map[string]string{launcherDrainTimeoutEnv: tc.envVal})
			if got := LauncherDrainTimeout(); got != tc.want {
				t.Errorf("LauncherDrainTimeout() = %v, want %v", got, tc.want)
			}
		})
	}
}
