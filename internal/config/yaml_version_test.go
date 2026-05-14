package config

import (
	"strings"
	"testing"
)

// TestLoadYAML_ParsesVersionField verifies the new top-level `version:` is
// captured by the loader so downstream callers can branch on it.
func TestLoadYAML_ParsesVersionField(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/raioz.yaml"
	yamlText := `version: "1"
project: test
services:
  api:
    path: ./api
`
	if err := writeTestFile(path, yamlText); err != nil {
		t.Fatalf("write: %v", err)
	}
	cfg, err := LoadYAML(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Version != "1" {
		t.Errorf("expected Version=%q, got %q", "1", cfg.Version)
	}
}

// TestLoadYAML_VersionOmitted_StillLoads verifies backward compatibility:
// configs without `version:` still parse cleanly.
func TestLoadYAML_VersionOmitted_StillLoads(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/raioz.yaml"
	yamlText := `project: test
services:
  api:
    path: ./api
`
	if err := writeTestFile(path, yamlText); err != nil {
		t.Fatalf("write: %v", err)
	}
	cfg, err := LoadYAML(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Version != "" {
		t.Errorf("expected Version empty when omitted, got %q", cfg.Version)
	}
}

// TestLoadDepsFromYAML_MissingVersionEmitsWarning checks the warning
// surfaces through the public load path so CLI commands can print it.
func TestLoadDepsFromYAML_MissingVersionEmitsWarning(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/raioz.yaml"
	yamlText := `project: test
services:
  api:
    path: ./api
`
	if err := writeTestFile(path, yamlText); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, warnings, err := LoadDepsFromYAML(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	found := false
	for _, w := range warnings {
		if strings.Contains(w, "version:") && strings.Contains(w, "versioning") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected a version-related warning, got: %v", warnings)
	}
}

// TestLoadDepsFromYAML_VersionPresentNoWarning ensures we don't pester
// users who already declared the field.
func TestLoadDepsFromYAML_VersionPresentNoWarning(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/raioz.yaml"
	yamlText := `version: "1"
project: test
services:
  api:
    path: ./api
`
	if err := writeTestFile(path, yamlText); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, warnings, err := LoadDepsFromYAML(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	for _, w := range warnings {
		if strings.Contains(w, "version:") {
			t.Errorf("unexpected version warning when field is present: %q", w)
		}
	}
}

// TestSchemaVersionWarnings_Direct exercises the helper in isolation so
// the warning text doesn't drift without a test catching it.
func TestSchemaVersionWarnings_Direct(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *RaiozConfig
		wantHit bool
	}{
		{"nil config", nil, false},
		{"missing version", &RaiozConfig{Project: "x"}, true},
		{"version set", &RaiozConfig{Project: "x", Version: "1"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := schemaVersionWarnings(tt.cfg)
			if tt.wantHit && len(got) == 0 {
				t.Error("expected a warning, got none")
			}
			if !tt.wantHit && len(got) != 0 {
				t.Errorf("expected no warnings, got: %v", got)
			}
		})
	}
}

// TestSchemaVersionWarnings_Mismatch guards ADR-031. The
// helper must emit version-mismatch warnings so a config declaring a
// schema this binary doesn't understand can't slip through silently.
// The wantSubstr fragment pins enough text to catch drift without
// reproducing the entire message.
func TestSchemaVersionWarnings_Mismatch(t *testing.T) {
	tests := []struct {
		name       string
		version    string
		wantSubstr string
	}{
		{"newer schema (v99)", "99", "supports version: \"1\""},
		{"newer schema (v2)", "2", "supports version: \"1\""},
		{"older schema (v0)", "0", "expects version: \"1\""},
		{"malformed (semver-shaped)", "1.0", "not a recognized schema number"},
		{"malformed (string)", "abc", "not a recognized schema number"},
		{"malformed (negative)", "-1", "not a recognized schema number"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := schemaVersionWarnings(&RaiozConfig{
				Project: "x", Version: tt.version,
			})
			if len(got) != 1 {
				t.Fatalf("expected exactly 1 warning, got %d: %v", len(got), got)
			}
			if !strings.Contains(got[0], tt.wantSubstr) {
				t.Errorf("warning missing expected fragment %q:\n  got: %s",
					tt.wantSubstr, got[0])
			}
		})
	}
}

// TestSchemaVersionWarnings_CurrentNoWarning is the inverse of the
// mismatch test: declaring the current version explicitly produces
// nothing. Belt-and-suspenders against a typo in the helper that
// might warn unconditionally.
func TestSchemaVersionWarnings_CurrentNoWarning(t *testing.T) {
	got := schemaVersionWarnings(&RaiozConfig{
		Project: "x", Version: CurrentSchemaVersion,
	})
	if len(got) != 0 {
		t.Errorf("expected no warnings for current version, got: %v", got)
	}
}

// TestCompareSchemaVersion documents the helper's expected
// comparison semantics. Strings like "v1" or "1.0" are NOT valid —
// the schema number is an integer.
func TestCompareSchemaVersion(t *testing.T) {
	tests := []struct {
		declared, current string
		wantCmp           int
		wantOK            bool
	}{
		{"1", "1", 0, true},
		{"2", "1", 1, true},
		{"0", "1", -1, true},
		{"  3  ", "1", 1, true}, // whitespace tolerated
		{"abc", "1", 0, false},
		{"1", "abc", 0, false},
		{"-1", "1", 0, false},
		{"", "1", 0, false},
		{"1.0", "1", 0, false},
		{"v1", "1", 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.declared+"_vs_"+tt.current, func(t *testing.T) {
			cmp, ok := compareSchemaVersion(tt.declared, tt.current)
			if ok != tt.wantOK {
				t.Errorf("ok = %v, want %v", ok, tt.wantOK)
			}
			if cmp != tt.wantCmp {
				t.Errorf("cmp = %d, want %d", cmp, tt.wantCmp)
			}
		})
	}
}
