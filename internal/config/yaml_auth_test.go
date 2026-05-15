package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestYAMLService_AuthField_Unmarshal verifies that `auth:` in yaml
// reaches both YAMLService.Auth and the bridged Service.Source.Auth.
// Empty / omitted maps to "" (strict default).
func TestYAMLService_AuthField_Unmarshal(t *testing.T) {
	cases := []struct {
		name     string
		yamlVal  string // raw yaml value for the auth field (or "" to omit)
		wantAuth string // expected Auth after bridging
	}{
		{"omitted maps to strict default", "", ""},
		{"inherit roundtrips", "inherit", "inherit"},
		{"gh roundtrips even before provider lands", "gh", "gh"},
		{"ssh roundtrips even before provider lands", "ssh", "ssh"},
		{"unknown value carries through", "garbage", "garbage"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			authLine := ""
			if tc.yamlVal != "" {
				authLine = "    auth: " + tc.yamlVal + "\n"
			}
			yamlSrc := "project: t\nservices:\n  api:\n    git: github.com/foo/bar\n    branch: main\n" + authLine

			dir := t.TempDir()
			path := filepath.Join(dir, "raioz.yaml")
			if err := os.WriteFile(path, []byte(yamlSrc), 0o644); err != nil {
				t.Fatalf("write: %v", err)
			}

			cfg, err := LoadYAML(path)
			if err != nil {
				t.Fatalf("LoadYAML: %v", err)
			}
			if got := cfg.Services["api"].Auth; got != tc.wantAuth {
				t.Errorf("YAMLService.Auth: want %q, got %q", tc.wantAuth, got)
			}

			deps, err := YAMLToDeps(cfg)
			if err != nil {
				t.Fatalf("YAMLToDeps: %v", err)
			}
			if got := deps.Services["api"].Source.Auth; got != tc.wantAuth {
				t.Errorf("SourceConfig.Auth: want %q, got %q", tc.wantAuth, got)
			}
		})
	}
}

// TestYAMLService_AuthField_OnlyForGitSources confirms a Path-only
// (non-git) service still parses an `auth:` value into YAMLService
// without crashing — but the bridge does NOT propagate it because the
// generated SourceConfig.Kind is "local", not "git". The validator
// in commit 6 will reject this combo, but unmarshal must stay clean.
func TestYAMLService_AuthField_OnlyForGitSources(t *testing.T) {
	yamlSrc := "project: t\nservices:\n  api:\n    path: ./api\n    auth: inherit\n"

	dir := t.TempDir()
	path := filepath.Join(dir, "raioz.yaml")
	if err := os.WriteFile(path, []byte(yamlSrc), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	cfg, err := LoadYAML(path)
	if err != nil {
		t.Fatalf("LoadYAML: %v", err)
	}
	if cfg.Services["api"].Auth != "inherit" {
		t.Errorf("YAMLService.Auth should carry the value: got %q",
			cfg.Services["api"].Auth)
	}

	deps, err := YAMLToDeps(cfg)
	if err != nil {
		t.Fatalf("YAMLToDeps: %v", err)
	}
	// Local services don't have Source.Auth populated — only the git
	// branch in the bridge sets it (by design). The validator will
	// reject this combo in commit 6.
	if deps.Services["api"].Source.Kind != "local" {
		t.Errorf("expected Kind=local for path-only service, got %q",
			deps.Services["api"].Source.Kind)
	}
	if deps.Services["api"].Source.Auth != "" {
		t.Errorf("local Source should not carry Auth; got %q",
			deps.Services["api"].Source.Auth)
	}
}
