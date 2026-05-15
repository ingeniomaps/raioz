package config

import (
	"testing"
)

func TestCheckImagePinning(t *testing.T) {
	cases := []struct {
		name     string
		image    string
		wantWarn bool
	}{
		// Pinned — no warning expected.
		{"empty", "", false},
		{"semver tag", "postgres:16", false},
		{"semver tag with variant", "postgres:16.3-alpine", false},
		{"non-semver but explicit tag", "postgres:bookworm", false},
		{"digest pinned", "postgres@sha256:abcdef0123456789", false},
		{"registry with digest", "registry.io/postgres@sha256:abc", false},
		{"registry with port and tag", "registry:5000/postgres:16", false},
		{"registry path with tag", "registry.io/org/postgres:16", false},
		{"deep registry path with tag", "registry.io/team/project/postgres:16", false},

		// Unpinned — warning expected.
		{"no tag bare name", "postgres", true},
		{"latest explicit", "postgres:latest", true},
		{"registry no tag", "registry.io/postgres", true},
		{"registry with port no tag", "registry:5000/postgres", true},
		{"deep registry no tag", "registry.io/team/postgres", true},
		{"latest with registry", "registry.io/postgres:latest", true},
		{"latest with port registry", "registry:5000/postgres:latest", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := checkImagePinning("dep", tc.image)
			gotWarn := got != ""
			if gotWarn != tc.wantWarn {
				t.Errorf("checkImagePinning(%q): warn=%v, want %v (got message=%q)",
					tc.image, gotWarn, tc.wantWarn, got)
			}
		})
	}
}

func TestCheckImagePinning_DistinguishesNoTagFromLatest(t *testing.T) {
	// The two warning paths must produce distinct keys so callers can
	// surface different guidance ("pin a tag" vs "stop using :latest").
	// Since i18n.T returns the key in test context, we check the
	// returned messages differ.
	noTag := checkImagePinning("dep", "postgres")
	latest := checkImagePinning("dep", "postgres:latest")
	if noTag == "" || latest == "" {
		t.Fatalf("expected both to produce warnings, got noTag=%q, latest=%q", noTag, latest)
	}
	if noTag == latest {
		t.Errorf("expected distinct warnings; both returned %q", noTag)
	}
}

func TestImagePinningWarnings_Empty(t *testing.T) {
	if got := imagePinningWarnings(nil); got != nil {
		t.Errorf("nil cfg should return nil, got %v", got)
	}
	if got := imagePinningWarnings(&RaiozConfig{}); got != nil {
		t.Errorf("empty cfg should return nil, got %v", got)
	}
}

func TestImagePinningWarnings_CleanConfig(t *testing.T) {
	cfg := &RaiozConfig{
		Deps: map[string]YAMLDependency{
			"postgres": {Image: "postgres:16"},
			"redis":    {Image: "redis:7.2"},
			"adminer":  {Compose: []string{"./adminer.yml"}}, // no image, compose-backed
		},
	}
	if got := imagePinningWarnings(cfg); len(got) != 0 {
		t.Errorf("clean config should produce no warnings, got %v", got)
	}
}

func TestImagePinningWarnings_AccumulatesAll(t *testing.T) {
	cfg := &RaiozConfig{
		Deps: map[string]YAMLDependency{
			"alpha":   {Image: "alpha"},         // no tag
			"beta":    {Image: "beta:latest"},   // latest
			"gamma":   {Image: "gamma:16"},      // pinned, no warning
			"delta":   {Image: "delta"},         // no tag
			"epsilon": {Compose: []string{"x"}}, // no image, no warning
		},
	}
	got := imagePinningWarnings(cfg)
	if len(got) != 3 {
		t.Fatalf("expected 3 warnings (alpha, beta, delta), got %d: %v", len(got), got)
	}
}

func TestImagePinningWarnings_DeterministicOrder(t *testing.T) {
	cfg := &RaiozConfig{
		Deps: map[string]YAMLDependency{
			"zeta":  {Image: "zeta"},
			"alpha": {Image: "alpha:latest"},
			"mu":    {Image: "mu"},
		},
	}
	// Run multiple times to surface any iteration-order non-determinism.
	var first []string
	for i := range 10 {
		got := imagePinningWarnings(cfg)
		if i == 0 {
			first = got
			continue
		}
		if len(got) != len(first) {
			t.Fatalf("iteration %d: length differs from first run", i)
		}
		for j := range got {
			if got[j] != first[j] {
				t.Errorf("iteration %d, position %d: %q != %q (non-deterministic order)",
					i, j, got[j], first[j])
			}
		}
	}
	// Sanity: 3 warnings total (alpha:latest + mu and zeta with no tag).
	// We can't assert dep-name ordering directly here because i18n.T
	// returns the bare key in test context (no arg interpolation), so
	// the warning text doesn't include the dep name. The cross-run
	// equality check above is what really exercises determinism.
	if len(first) != 3 {
		t.Errorf("expected 3 warnings, got %d", len(first))
	}
}
