package app

import (
	"os"
	"slices"
	"strings"
	"testing"

	"raioz/internal/domain/models"
)

// TestBuildStopCmdEnv_InheritsParentEnv is the regression guard for
// issue 044: the custom `stop:` command previously ran with an empty
// `cmd.Env`, so the child shell had no PATH and `make
// dev-docker-stop` failed to find `docker`. The fix initializes
// `cmd.Env = os.Environ()` before appending RAIOZ_ENV_FILE entries.
// This test asserts the parent env survives that path.
func TestBuildStopCmdEnv_InheritsParentEnv(t *testing.T) {
	// Set a sentinel env var so the test doesn't rely on PATH being
	// non-empty (some CI sandboxes scrub PATH). os.Setenv via t.Setenv
	// is reverted after the test.
	t.Setenv("RAIOZ_TEST_SENTINEL", "yes")

	got := buildStopCmdEnv(models.Service{})

	if !slices.Contains(got, "RAIOZ_TEST_SENTINEL=yes") {
		t.Fatalf("buildStopCmdEnv() did not inherit parent env: %v", got)
	}
	// And it must not be empty in general — a previous regression
	// returned nil even with no env files declared.
	if len(got) == 0 {
		t.Fatal("buildStopCmdEnv() returned empty slice; expected at least the inherited env")
	}
}

// TestBuildStopCmdEnv_AppendsEnvFiles documents the override
// semantics: env files declared on the service produce
// RAIOZ_ENV_FILE entries appended after the inherited block. The
// child process sees both; Go's exec lets the last entry win for
// duplicate keys.
func TestBuildStopCmdEnv_AppendsEnvFiles(t *testing.T) {
	t.Setenv("RAIOZ_TEST_SENTINEL", "yes")

	svc := models.Service{
		Env: &models.EnvValue{
			Files: []string{".env.local", "", ".env.shared"},
		},
	}
	got := buildStopCmdEnv(svc)

	wantParent := "RAIOZ_TEST_SENTINEL=yes"
	if !slices.Contains(got, wantParent) {
		t.Errorf("missing inherited entry %q: %v", wantParent, got)
	}

	want := []string{
		"RAIOZ_ENV_FILE=.env.local",
		"RAIOZ_ENV_FILE=.env.shared",
	}
	for _, w := range want {
		if !slices.Contains(got, w) {
			t.Errorf("missing env-file entry %q in: %v", w, got)
		}
	}

	// Empty-string entries are filtered out — neither
	// "RAIOZ_ENV_FILE=" nor a bare "" should appear.
	if slices.Contains(got, "RAIOZ_ENV_FILE=") {
		t.Error("buildStopCmdEnv() leaked an empty RAIOZ_ENV_FILE entry")
	}

	// RAIOZ_ENV_FILE entries must come after the inherited block.
	// Find the index of the first env-file entry and assert at least
	// one parent-env entry exists before it.
	firstOverride := -1
	for i, e := range got {
		if strings.HasPrefix(e, "RAIOZ_ENV_FILE=") {
			firstOverride = i
			break
		}
	}
	if firstOverride <= 0 {
		t.Errorf("RAIOZ_ENV_FILE entries must come after inherited env; first at index %d", firstOverride)
	}
}

// TestBuildStopCmdEnv_NoEnvFiles documents the nil-env path: when
// the service declares no env section, the function returns exactly
// the inherited environment (no extra entries).
func TestBuildStopCmdEnv_NoEnvFiles(t *testing.T) {
	got := buildStopCmdEnv(models.Service{})
	if len(got) != len(os.Environ()) {
		t.Errorf("buildStopCmdEnv() with nil Env = %d entries, want %d (parent env)",
			len(got), len(os.Environ()))
	}
}
