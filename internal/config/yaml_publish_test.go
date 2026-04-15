package config

import (
	"os"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// Tests for YAMLPublish / YAMLIntSlice unmarshalling and the bridge mapping
// into Infra. Keeps the YAML surface honest: future refactors of the custom
// unmarshallers stay within these shapes.

func parseDep(t *testing.T, yamlText string) YAMLDependency {
	t.Helper()
	var dep YAMLDependency
	if err := yaml.Unmarshal([]byte(yamlText), &dep); err != nil {
		t.Fatalf("yaml parse: %v\ninput:\n%s", err, yamlText)
	}
	return dep
}

func TestYAMLDependency_PublishBool(t *testing.T) {
	dep := parseDep(t, "image: postgres:16\npublish: true\n")
	if !dep.Publish.Set {
		t.Error("Publish.Set should be true")
	}
	if !dep.Publish.Auto {
		t.Error("Publish.Auto should be true")
	}
	if len(dep.Publish.Ports) != 0 {
		t.Errorf("Publish.Ports = %v, want empty", dep.Publish.Ports)
	}
}

func TestYAMLDependency_PublishFalse(t *testing.T) {
	dep := parseDep(t, "image: postgres:16\npublish: false\n")
	if !dep.Publish.Set {
		t.Error("Publish.Set should be true (user wrote the field)")
	}
	if dep.Publish.Auto {
		t.Error("Publish.Auto should be false")
	}
}

func TestYAMLDependency_PublishSingleInt(t *testing.T) {
	dep := parseDep(t, "image: postgres:16\npublish: 5432\n")
	if !dep.Publish.Set {
		t.Error("Publish.Set should be true")
	}
	if dep.Publish.Auto {
		t.Error("Publish.Auto should be false for int form")
	}
	if len(dep.Publish.Ports) != 1 || dep.Publish.Ports[0] != 5432 {
		t.Errorf("Publish.Ports = %v, want [5432]", dep.Publish.Ports)
	}
}

func TestYAMLDependency_PublishList(t *testing.T) {
	dep := parseDep(t, "image: fake\npublish: [5432, 9090]\n")
	if len(dep.Publish.Ports) != 2 ||
		dep.Publish.Ports[0] != 5432 ||
		dep.Publish.Ports[1] != 9090 {
		t.Errorf("Publish.Ports = %v, want [5432 9090]", dep.Publish.Ports)
	}
}

func TestYAMLDependency_PublishUnset(t *testing.T) {
	dep := parseDep(t, "image: postgres:16\n")
	if dep.Publish.Set {
		t.Error("Publish.Set should be false when field missing")
	}
}

func TestYAMLDependency_ExposeSingleInt(t *testing.T) {
	dep := parseDep(t, "image: postgres:16\nexpose: 5432\n")
	if len(dep.Expose) != 1 || dep.Expose[0] != 5432 {
		t.Errorf("Expose = %v, want [5432]", dep.Expose)
	}
}

func TestYAMLDependency_ExposeList(t *testing.T) {
	dep := parseDep(t, "image: fake\nexpose: [5432, 9090]\n")
	if len(dep.Expose) != 2 {
		t.Fatalf("Expose = %v, want 2 entries", dep.Expose)
	}
}

func TestYAMLBridge_PublishAutoPropagates(t *testing.T) {
	cfg := &RaiozConfig{
		Project: "test",
		Deps: map[string]YAMLDependency{
			"postgres": parseDep(t, "image: postgres:16\nexpose: 5432\npublish: true\n"),
		},
	}
	deps, err := YAMLToDeps(cfg)
	if err != nil {
		t.Fatalf("YAMLToDeps: %v", err)
	}
	entry := deps.Infra["postgres"]
	if entry.Inline == nil {
		t.Fatal("expected inline infra")
	}
	if entry.Inline.Publish == nil {
		t.Fatal("Publish should not be nil when user declared publish: true")
	}
	if !entry.Inline.Publish.Auto {
		t.Error("Publish.Auto should be true after bridge")
	}
	if len(entry.Inline.Expose) != 1 || entry.Inline.Expose[0] != 5432 {
		t.Errorf("Expose not propagated: %v", entry.Inline.Expose)
	}
}

func TestYAMLBridge_PublishExplicitPropagates(t *testing.T) {
	cfg := &RaiozConfig{
		Project: "test",
		Deps: map[string]YAMLDependency{
			"postgres": parseDep(t, "image: postgres:16\npublish: 5432\n"),
		},
	}
	deps, err := YAMLToDeps(cfg)
	if err != nil {
		t.Fatalf("YAMLToDeps: %v", err)
	}
	pub := deps.Infra["postgres"].Inline.Publish
	if pub == nil || pub.Auto || len(pub.Ports) != 1 || pub.Ports[0] != 5432 {
		t.Errorf("Publish = %+v, want explicit [5432]", pub)
	}
}

func TestYAMLBridge_NoPublishYieldsNilSpec(t *testing.T) {
	cfg := &RaiozConfig{
		Project: "test",
		Deps: map[string]YAMLDependency{
			"postgres": parseDep(t, "image: postgres:16\n"),
		},
	}
	deps, _ := YAMLToDeps(cfg)
	if deps.Infra["postgres"].Inline.Publish != nil {
		t.Error("Publish should be nil when user didn't write the field")
	}
}

func TestYAMLBridge_LegacyPortsStillWorks(t *testing.T) {
	// Backwards compatibility: the old ports: [...] form still produces
	// Infra.Ports so existing .raioz.json / raioz.yaml configs don't break.
	cfg := &RaiozConfig{
		Project: "test",
		Deps: map[string]YAMLDependency{
			"postgres": parseDep(t,
				"image: postgres:16\nports: [\"5432:5432\"]\n"),
		},
	}
	deps, _ := YAMLToDeps(cfg)
	if got := deps.Infra["postgres"].Inline.Ports; len(got) != 1 ||
		got[0] != "5432:5432" {
		t.Errorf("legacy ports = %v, want [5432:5432]", got)
	}
}

func TestLoadDepsFromYAML_LegacyPortsEmitsWarning(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/raioz.yaml"
	yamlText := `project: test
dependencies:
  postgres:
    image: postgres:16
    ports: ["5432:5432"]
`
	if err := writeTestFile(path, yamlText); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, warnings, err := LoadDepsFromYAML(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(warnings) == 0 {
		t.Fatal("expected at least one warning for legacy ports")
	}
	found := false
	for _, w := range warnings {
		if containsAll(w, "postgres", "ports", "publish") {
			found = true
		}
	}
	if !found {
		t.Errorf("warnings don't mention the legacy→publish migration: %v", warnings)
	}
}

// TestLoadDepsFromYAML_UnknownFieldEmitsWarning: typo'd or newer-schema
// fields must surface as advisory warnings (not errors) so users catch
// silent drops like the v0.1.0 `dependencies.<n>.proxy:` bug at load time.
func TestLoadDepsFromYAML_UnknownFieldEmitsWarning(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/raioz.yaml"
	yamlText := `project: test
services:
  api:
    path: ./api
    whtch: true
`
	if err := writeTestFile(path, yamlText); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, warnings, err := LoadDepsFromYAML(path)
	if err != nil {
		t.Fatalf("unknown fields must NOT fail the load: %v", err)
	}
	found := false
	for _, w := range warnings {
		if containsAll(w, "whtch") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected warning mentioning the unknown field `whtch`, got: %v", warnings)
	}
}

// TestLoadDepsFromYAML_NoUnknownFields_NoWarning: happy path stays quiet —
// a well-formed config emits zero unknown-field warnings (deprecation
// warnings unrelated to this helper are tested separately).
func TestLoadDepsFromYAML_NoUnknownFields_NoWarning(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/raioz.yaml"
	yamlText := `project: test
services:
  api:
    path: ./api
dependencies:
  postgres:
    image: postgres:16
`
	if err := writeTestFile(path, yamlText); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, warnings, err := LoadDepsFromYAML(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	for _, w := range warnings {
		if strings.Contains(strings.ToLower(w), "field") &&
			strings.Contains(strings.ToLower(w), "not found") {
			t.Errorf("unexpected unknown-field warning on clean config: %s", w)
		}
	}
}

// --- tiny helpers that keep the tests self-contained ------------------------

func writeTestFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o600)
}

func containsAll(s string, subs ...string) bool {
	lower := strings.ToLower(s)
	for _, sub := range subs {
		if !strings.Contains(lower, strings.ToLower(sub)) {
			return false
		}
	}
	return true
}
