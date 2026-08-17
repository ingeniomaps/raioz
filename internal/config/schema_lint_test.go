package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func lintMetas(t *testing.T) []FieldMeta {
	t.Helper()
	metas, err := ExtractFieldMetaEmbedded()
	if err != nil {
		t.Fatalf("ExtractFieldMetaEmbedded: %v", err)
	}
	if len(metas) == 0 {
		t.Fatal("no field metadata extracted; the lint would report nothing")
	}
	return metas
}

func paths(findings []LintFinding) []string {
	out := make([]string, 0, len(findings))
	for _, f := range findings {
		out = append(out, f.Path)
	}
	return out
}

// `raioz yaml lint` prints one line per finding. The walk reaches
// services and dependencies through map iteration, which Go randomizes,
// so without an explicit sort the same config printed a different order
// on every run and two outputs could not be diffed.
func TestLintConfigOrderIsStable(t *testing.T) {
	metas := lintMetas(t)
	cfg := &RaiozConfig{
		Version: "1",
		Project: "p",
		Services: map[string]YAMLService{
			"alpha": {Path: "./a"},
			"beta":  {Path: "./b"},
			"gamma": {Path: "./c"},
			"delta": {Path: "./d"},
			"eps":   {Path: "./e"},
		},
	}

	first := paths(LintConfig(cfg, metas, "1"))
	for run := 1; run < 12; run++ {
		got := paths(LintConfig(cfg, metas, "1"))
		if len(got) != len(first) {
			t.Fatalf("run %d returned %d findings, first returned %d", run, len(got), len(first))
		}
		for i := range got {
			if got[i] != first[i] {
				t.Fatalf("run %d differs at %d: %q vs %q", run, i, got[i], first[i])
			}
		}
	}

	for i := 1; i < len(first); i++ {
		if first[i-1] > first[i] {
			t.Errorf("findings must come out sorted: %q before %q", first[i-1], first[i])
		}
	}
}

// The lint reports on what the config *uses*, not on everything the
// schema offers — otherwise every run would bury the real signal under
// the full field list.
func TestLintConfigReportsOnlyPopulatedFields(t *testing.T) {
	metas := lintMetas(t)
	cfg := &RaiozConfig{Version: "1", Project: "p"}

	got := paths(LintConfig(cfg, metas, "1"))

	for _, p := range got {
		if p != "version" && p != "project" {
			t.Errorf("unpopulated field reported: %q", p)
		}
	}
	if len(got) != 2 {
		t.Errorf("expected version and project only, got %v", got)
	}
}

// The whole point of the warn level: a config that uses versioned fields
// without declaring `version:` gets told to declare one.
func TestLintConfigSeverityDependsOnDeclaredVersion(t *testing.T) {
	metas := lintMetas(t)
	cfg := &RaiozConfig{Project: "p", Services: map[string]YAMLService{"api": {Path: "./api"}}}

	t.Run("no version declared", func(t *testing.T) {
		for _, f := range LintConfig(cfg, metas, "") {
			if f.Severity != "warn" {
				t.Errorf("%s severity = %q, want warn", f.Path, f.Severity)
			}
			if !strings.Contains(f.Message, "version:") {
				t.Errorf("%s message should tell the user to declare a version: %q", f.Path, f.Message)
			}
		}
	})

	t.Run("version declared", func(t *testing.T) {
		findings := LintConfig(cfg, metas, "1")
		if len(findings) == 0 {
			t.Fatal("expected findings for a populated config")
		}
		for _, f := range findings {
			if f.Severity != "ok" {
				t.Errorf("%s severity = %q, want ok", f.Path, f.Severity)
			}
			if f.Since == "" || !strings.Contains(f.Message, f.Since) {
				t.Errorf("%s should show its since marker, got %q", f.Path, f.Message)
			}
		}
	})
}

// Services and dependencies are maps of structs; each entry has to
// produce a dotted path the user can find in their own file.
func TestLintConfigWalksNestedShapes(t *testing.T) {
	metas := lintMetas(t)
	publish := false
	cfg := &RaiozConfig{
		Version: "1",
		Project: "p",
		Proxy:   &ProxyConfig{Enabled: true, Domain: "acme.dev", Publish: &publish},
		Services: map[string]YAMLService{
			"api": {Path: "./api", Hostname: "api", HostnameAliases: YAMLStringSlice{"www"}},
		},
		Deps: map[string]YAMLDependency{
			"db": {Image: "postgres:16"},
		},
	}

	got := paths(LintConfig(cfg, metas, "1"))
	joined := strings.Join(got, " ")

	for _, want := range []string{
		"services.api.hostnameAliases", // map of structs
		"dependencies.db.image",        // second map
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("expected a finding for %q, got %v", want, got)
		}
	}

	// Documented gap: only yaml_types.go is embedded, so fields declared
	// in internal/domain/models (ProxyConfig, RoutingConfig) carry no
	// metadata and the walk reports `proxy` without descending into it.
	// If this starts failing, ExtractFieldMetaEmbedded grew a source and
	// the doc comment on it needs updating too.
	if strings.Contains(joined, "proxy.") {
		t.Errorf("proxy sub-fields are not extracted today; got %v", got)
	}
}

// A nil pointer field is not "used", and descending into it would panic.
func TestLintConfigSkipsNilPointers(t *testing.T) {
	metas := lintMetas(t)
	cfg := &RaiozConfig{Version: "1", Project: "p", Proxy: nil}

	for _, p := range paths(LintConfig(cfg, metas, "1")) {
		if strings.HasPrefix(p, "proxy") {
			t.Errorf("nil proxy must not be walked, got %q", p)
		}
	}
}

func TestLintConfigNilConfig(t *testing.T) {
	if got := LintConfig(nil, lintMetas(t), "1"); got != nil {
		t.Errorf("nil config = %v, want nil findings", got)
	}
}

// LintConfigPath is what the CLI calls: it has to surface a load failure
// rather than lint an empty config.
func TestLintConfigPath(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "raioz.yaml")
	yaml := "version: \"1\"\nproject: demo\nservices:\n  api:\n    path: ./api\n"
	if err := os.WriteFile(good, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}

	findings, cfg, err := LintConfigPath(good)
	if err != nil {
		t.Fatalf("LintConfigPath: %v", err)
	}
	if cfg == nil || cfg.Project != "demo" {
		t.Fatalf("config not returned: %+v", cfg)
	}
	if len(findings) == 0 {
		t.Error("a config that declares project and version should report both")
	}

	if _, _, err := LintConfigPath(filepath.Join(dir, "missing.yaml")); err == nil {
		t.Error("a missing file must surface an error")
	}
}
