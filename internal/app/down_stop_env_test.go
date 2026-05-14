package app

import (
	"os"
	"slices"
	"strings"
	"testing"

	"raioz/internal/domain/models"
)

// Regression guard: the stop env must inherit the parent's.
// Sentinel var avoids relying on PATH (some CI sandboxes scrub it).
func TestBuildStopCmdEnv_InheritsParentEnv(t *testing.T) {
	t.Setenv("RAIOZ_TEST_SENTINEL", "yes")

	got := buildStopCmdEnv(models.Service{})

	if !slices.Contains(got, "RAIOZ_TEST_SENTINEL=yes") {
		t.Fatalf("buildStopCmdEnv() did not inherit parent env: %v", got)
	}
	if len(got) == 0 {
		t.Fatal("buildStopCmdEnv() returned empty slice; expected at least the inherited env")
	}
}

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

	if slices.Contains(got, "RAIOZ_ENV_FILE=") {
		t.Error("buildStopCmdEnv() leaked an empty RAIOZ_ENV_FILE entry")
	}

	// Overrides must come after inherited entries so duplicate keys
	// resolve to the override value.
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

func TestBuildStopCmdEnv_NoEnvFiles(t *testing.T) {
	got := buildStopCmdEnv(models.Service{})
	if len(got) != len(os.Environ()) {
		t.Errorf("buildStopCmdEnv() with nil Env = %d entries, want %d (parent env)",
			len(got), len(os.Environ()))
	}
}
