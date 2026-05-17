package host

import (
	"os"
	"sync"
	"time"

	"raioz/internal/logging"
)

// Duration-typed env vars raioz reads. New knobs MUST be appended to
// KnownDurationEnvs() below or they inherit the silent-fallback bug
// (ADR-035).
const (
	launcherWaitTimeoutEnv  = "RAIOZ_LAUNCHER_TIMEOUT"
	launcherDrainTimeoutEnv = "RAIOZ_LAUNCHER_DRAIN_TIMEOUT"
	siblingSpawnTimeoutEnv  = "RAIOZ_SIBLING_TIMEOUT"
	lockStaleAgeEnv         = "RAIOZ_LOCK_STALE_AGE"
)

// defaultLockStaleAge is the floor that evicts a lock whose PID is
// alive but was reused by a non-raioz process (PID wraparound, common
// in containers with low pid_max). 24h is generous; CI runners that
// timeout in 30m and SIGKILL a raioz mid-up can tune RAIOZ_LOCK_STALE_AGE
// down to e.g. `30m` to recover faster.
const defaultLockStaleAge = 24 * time.Hour

// LauncherWaitTimeout — post-launcher container-appearance wait
// during `raioz up`. ADR-025.
func LauncherWaitTimeout() time.Duration {
	return durationFromEnv(launcherWaitTimeoutEnv, 60*time.Second)
}

// LauncherDrainTimeout — wait for an in-progress launcher build
// during `raioz down` before invoking `stop:`. ADR-025.
func LauncherDrainTimeout() time.Duration {
	return durationFromEnv(launcherDrainTimeoutEnv, 30*time.Second)
}

// SiblingSpawnTimeout caps each mode-A `raioz up` child. Default 10m
// is 5× the typical 30s–2m spawn; bump RAIOZ_SIBLING_TIMEOUT for
// projects with heavy `pre:` hooks.
func SiblingSpawnTimeout() time.Duration {
	return durationFromEnv(siblingSpawnTimeoutEnv, 10*time.Minute)
}

// LockStaleAge — RAIOZ_LOCK_STALE_AGE knob from issue 029. Lock
// package consults this for the age-based eviction floor.
func LockStaleAge() time.Duration {
	return durationFromEnv(lockStaleAgeEnv, defaultLockStaleAge)
}

// EnvDurationStatus snapshots how a duration-typed env var resolved.
// Used by `raioz doctor` to surface user overrides and malformed
// values that durationFromEnv silently masked behind the default.
// See ADR-035.
type EnvDurationStatus struct {
	Name      string        // env var name (e.g. RAIOZ_LAUNCHER_TIMEOUT)
	Raw       string        // user-supplied value; "" when unset
	Resolved  time.Duration // value actually used
	Default   time.Duration
	Malformed bool // true when Raw was set but couldn't parse / was negative
}

// InspectDurationEnv reads a duration env var WITHOUT logging side
// effects. Inverse of durationFromEnv when callers need to render
// the resolution state (`raioz doctor`) instead of just consuming
// the resolved value.
func InspectDurationEnv(name string, def time.Duration) EnvDurationStatus {
	raw := osGetenv(name)
	if raw == "" {
		return EnvDurationStatus{Name: name, Resolved: def, Default: def}
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d < 0 {
		return EnvDurationStatus{
			Name: name, Raw: raw, Resolved: def, Default: def, Malformed: true,
		}
	}
	return EnvDurationStatus{Name: name, Raw: raw, Resolved: d, Default: def}
}

// KnownDurationEnvs enumerates every duration-typed env var raioz
// reads. `raioz doctor` walks this list to print resolution state.
// New duration env vars MUST be appended here so the doctor surfaces
// them — otherwise a typo'd value stays silent. See ADR-035.
func KnownDurationEnvs() []EnvDurationStatus {
	return []EnvDurationStatus{
		InspectDurationEnv(launcherWaitTimeoutEnv, 60*time.Second),
		InspectDurationEnv(launcherDrainTimeoutEnv, 30*time.Second),
		InspectDurationEnv(siblingSpawnTimeoutEnv, 10*time.Minute),
		InspectDurationEnv(lockStaleAgeEnv, defaultLockStaleAge),
	}
}

// warnedEnvOnce tracks env vars we've already warned about so a
// hot loop reading the same malformed var doesn't spam the log.
// Per-process scope; tests reset via ResetMalformedEnvWarningsForTest.
var warnedEnvOnce sync.Map

// ResetMalformedEnvWarningsForTest clears the once-per-process
// dedup so tests can verify the warning fires on the first hit
// without depending on test ordering. Test-only.
func ResetMalformedEnvWarningsForTest() {
	warnedEnvOnce = sync.Map{}
}

// "0s" is honored as explicit opt-out; "" falls back to def; an
// unparseable or negative value also returns def but logs a warning
// once per (process, var name) so the user spots typos like
// "RAIOZ_LAUNCHER_TIMEOUT=60" (missing "s") instead of seeing the
// default silently. See ADR-035.
func durationFromEnv(name string, def time.Duration) time.Duration {
	s := InspectDurationEnv(name, def)
	if s.Malformed {
		if _, loaded := warnedEnvOnce.LoadOrStore(name, true); !loaded {
			logging.Warn("invalid duration env var; using default",
				"var", name,
				"value", s.Raw,
				"default", def.String(),
				"hint", "expected Go duration like 60s, 2m, 1h",
			)
		}
	}
	return s.Resolved
}

// Indirection seam for tests; never reassigned in non-test code.
var osGetenv = os.Getenv
