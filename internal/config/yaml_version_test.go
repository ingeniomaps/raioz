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
