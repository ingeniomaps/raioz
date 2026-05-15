package config

import (
	"strings"
	"testing"
)

func TestValidateAuthValues_Accepts(t *testing.T) {
	for _, val := range []string{"", "inherit", "gh", "ssh"} {
		t.Run(val, func(t *testing.T) {
			cfg := &RaiozConfig{
				Project: "t",
				Services: map[string]YAMLService{
					"api": {Path: "./api", Auth: val},
				},
			}
			if err := validateAuthValues(cfg, "test.yaml"); err != nil {
				t.Errorf("auth=%q should be accepted; got %v", val, err)
			}
		})
	}
}

func TestValidateAuthValues_Rejects(t *testing.T) {
	// Each case is a string we deliberately reject: typos, aliases,
	// case variants (yaml fields are case-sensitive), whitespace.
	cases := []string{
		"github",
		"GH",
		"INHERIT",
		"garbage",
		" gh",
		"gh ",
		"inherit ",
		"unknown",
		"INHERIT",
	}
	for _, val := range cases {
		t.Run(val, func(t *testing.T) {
			cfg := &RaiozConfig{
				Project: "t",
				Services: map[string]YAMLService{
					"api": {Path: "./api", Auth: val},
				},
			}
			err := validateAuthValues(cfg, "test.yaml")
			if err == nil {
				t.Fatalf("auth=%q should be rejected; got nil", val)
			}
			// Error should mention the bad value AND the service name
			// — devs need both to fix it.
			if !strings.Contains(err.Error(), val) {
				t.Errorf("error %q should mention bad value %q",
					err.Error(), val)
			}
			if !strings.Contains(err.Error(), "api") {
				t.Errorf("error %q should name the service", err.Error())
			}
		})
	}
}

func TestValidateAuthValues_NilCfg(t *testing.T) {
	if err := validateAuthValues(nil, "test.yaml"); err != nil {
		t.Errorf("nil cfg should return nil, got %v", err)
	}
}

func TestAuthWarnings_NilCfg(t *testing.T) {
	if got := authWarnings(nil); got != nil {
		t.Errorf("nil cfg should return nil, got %v", got)
	}
}

func TestAuthWarnings_AuthWithoutGit(t *testing.T) {
	cfg := &RaiozConfig{
		Services: map[string]YAMLService{
			// auth: declared but no git: — this is the silent-drop case.
			"api": {Path: "./api", Auth: "inherit"},
		},
	}
	warnings := authWarnings(cfg)
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d: %v", len(warnings), warnings)
	}
	if !strings.Contains(warnings[0], "api") {
		t.Errorf("warning should name the service; got %q", warnings[0])
	}
	if !strings.Contains(warnings[0], "inherit") {
		t.Errorf("warning should echo the auth value; got %q", warnings[0])
	}
}

func TestAuthWarnings_AuthWithGitIsClean(t *testing.T) {
	cfg := &RaiozConfig{
		Services: map[string]YAMLService{
			"api": {Git: "github.com/foo/bar", Branch: "main", Auth: "inherit"},
		},
	}
	if warnings := authWarnings(cfg); len(warnings) != 0 {
		t.Errorf("auth with git should not warn; got %v", warnings)
	}
}

func TestAuthWarnings_NoAuthIsClean(t *testing.T) {
	cfg := &RaiozConfig{
		Services: map[string]YAMLService{
			"api": {Path: "./api"},
			"web": {Git: "github.com/foo/web", Branch: "main"},
		},
	}
	if warnings := authWarnings(cfg); len(warnings) != 0 {
		t.Errorf("no auth should not warn; got %v", warnings)
	}
}

// TestAuthWarnings_DeterministicOrder pins the iteration order
// (sorted by service name) so warning output stays stable across
// runs — map iteration in Go is randomized.
func TestAuthWarnings_DeterministicOrder(t *testing.T) {
	cfg := &RaiozConfig{
		Services: map[string]YAMLService{
			"zeta":  {Path: "./z", Auth: "inherit"},
			"alpha": {Path: "./a", Auth: "gh"},
			"mu":    {Path: "./m", Auth: "ssh"},
		},
	}
	var first []string
	for i := range 10 {
		got := authWarnings(cfg)
		if i == 0 {
			first = got
			continue
		}
		if len(got) != len(first) {
			t.Fatalf("iteration %d: length differs", i)
		}
		for j := range got {
			if got[j] != first[j] {
				t.Errorf("iteration %d, pos %d: %q != %q (non-deterministic)",
					i, j, got[j], first[j])
			}
		}
	}
	if len(first) != 3 {
		t.Errorf("expected 3 warnings, got %d", len(first))
	}
}
