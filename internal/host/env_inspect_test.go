package host

import (
	"testing"
	"time"
)

func TestInspectDurationEnv_Unset(t *testing.T) {
	stubEnv(t, map[string]string{})
	got := InspectDurationEnv("FOO", 30*time.Second)
	if got.Raw != "" || got.Malformed {
		t.Errorf("unset must report Raw=\"\" Malformed=false, got %+v", got)
	}
	if got.Resolved != 30*time.Second {
		t.Errorf("Resolved = %v, want 30s", got.Resolved)
	}
}

func TestInspectDurationEnv_ValidOverride(t *testing.T) {
	stubEnv(t, map[string]string{"FOO": "2m"})
	got := InspectDurationEnv("FOO", 30*time.Second)
	if got.Malformed {
		t.Errorf("valid value must not be Malformed: %+v", got)
	}
	if got.Resolved != 2*time.Minute {
		t.Errorf("Resolved = %v, want 2m", got.Resolved)
	}
	if got.Raw != "2m" {
		t.Errorf("Raw = %q, want 2m", got.Raw)
	}
}

func TestInspectDurationEnv_Malformed(t *testing.T) {
	cases := map[string]string{
		"unit missing":    "60",
		"alphabetic":      "soon",
		"semver-shaped":   "1.0",
		"negative number": "-5s",
		"whitespace":      "  ",
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			stubEnv(t, map[string]string{"FOO": raw})
			got := InspectDurationEnv("FOO", 30*time.Second)
			if !got.Malformed {
				t.Errorf("%q should be Malformed, got %+v", raw, got)
			}
			if got.Resolved != 30*time.Second {
				t.Errorf("malformed must fall back to default: got %v", got.Resolved)
			}
			if got.Raw != raw {
				t.Errorf("Raw must be preserved verbatim: got %q want %q", got.Raw, raw)
			}
		})
	}
}

func TestDurationFromEnv_WarnsOncePerVar(t *testing.T) {
	// We can't easily intercept slog output without a logger sink,
	// but we CAN verify the dedup map records the var. That's the
	// observable contract: a second call sees the entry and skips
	// the log emit.
	ResetMalformedEnvWarningsForTest()
	stubEnv(t, map[string]string{
		launcherWaitTimeoutEnv: "60", // missing unit — malformed
	})

	_ = LauncherWaitTimeout()
	if _, ok := warnedEnvOnce.Load(launcherWaitTimeoutEnv); !ok {
		t.Error("first malformed read must register in warnedEnvOnce")
	}

	// Second call must not panic and must not re-register.
	_ = LauncherWaitTimeout()
	count := 0
	warnedEnvOnce.Range(func(_, _ any) bool { count++; return true })
	if count != 1 {
		t.Errorf("expected exactly one entry in warnedEnvOnce, got %d", count)
	}
}

func TestKnownDurationEnvs_CoversBothLauncherVars(t *testing.T) {
	stubEnv(t, map[string]string{})
	got := KnownDurationEnvs()
	names := map[string]bool{}
	for _, s := range got {
		names[s.Name] = true
	}
	if !names[launcherWaitTimeoutEnv] || !names[launcherDrainTimeoutEnv] {
		t.Errorf("KnownDurationEnvs must list both launcher vars; got %v", names)
	}
	if !names[siblingSpawnTimeoutEnv] {
		t.Errorf("KnownDurationEnvs must list RAIOZ_SIBLING_TIMEOUT (issue 072); got %v", names)
	}
}

func TestSiblingSpawnTimeout_DefaultAndOverride(t *testing.T) {
	t.Run("default 10 minutes when unset", func(t *testing.T) {
		stubEnv(t, map[string]string{})
		if got := SiblingSpawnTimeout(); got != 10*time.Minute {
			t.Errorf("default = %s, want 10m", got)
		}
	})
	t.Run("override honored", func(t *testing.T) {
		stubEnv(t, map[string]string{siblingSpawnTimeoutEnv: "2m30s"})
		if got := SiblingSpawnTimeout(); got != 2*time.Minute+30*time.Second {
			t.Errorf("override = %s, want 2m30s", got)
		}
	})
}

func TestKnownDurationEnvs_PropagatesMalformed(t *testing.T) {
	stubEnv(t, map[string]string{
		launcherWaitTimeoutEnv:  "abc",
		launcherDrainTimeoutEnv: "30s",
	})
	got := KnownDurationEnvs()
	var bad, good *EnvDurationStatus
	for i := range got {
		switch got[i].Name {
		case launcherWaitTimeoutEnv:
			bad = &got[i]
		case launcherDrainTimeoutEnv:
			good = &got[i]
		}
	}
	if bad == nil || !bad.Malformed {
		t.Errorf("malformed env must surface in KnownDurationEnvs: %+v", bad)
	}
	if good == nil || good.Malformed {
		t.Errorf("valid env must not be Malformed: %+v", good)
	}
}
