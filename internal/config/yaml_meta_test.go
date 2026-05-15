package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeMeta(t *testing.T, dir, body string) string {
	t.Helper()
	path := filepath.Join(dir, "raioz.yaml")
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return path
}

// A regular project config must be reported as "not meta" so the standard
// loader can take over — failing this means LoadMetaConfig swallowed
// non-meta files.
func TestLoadMetaConfig_RegularProjectIsNotMeta(t *testing.T) {
	dir := t.TempDir()
	path := writeMeta(t, dir, "project: ordinary\n")
	cfg, isMeta, err := LoadMetaConfig(path)
	if err != nil {
		t.Fatalf("LoadMetaConfig: %v", err)
	}
	if isMeta {
		t.Errorf("regular project misidentified as meta")
	}
	if cfg != nil {
		t.Errorf("cfg = %+v on non-meta config, want nil", cfg)
	}
}

// kind: meta + projects: → success, paths absolute, default order preserved.
func TestLoadMetaConfig_HappyPath(t *testing.T) {
	dir := t.TempDir()
	for _, sub := range []string{"keycloak", "api", "ui/portal"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0755); err != nil {
			t.Fatal(err)
		}
	}
	path := writeMeta(t, dir, `
workspace: hypixo
kind: meta
projects:
  - path: keycloak
  - path: api
  - path: ui/portal
    optional: true
`)
	cfg, isMeta, err := LoadMetaConfig(path)
	if err != nil {
		t.Fatalf("LoadMetaConfig: %v", err)
	}
	if !isMeta || cfg == nil {
		t.Fatal("expected meta config")
	}
	if cfg.Workspace != "hypixo" {
		t.Errorf("Workspace = %q", cfg.Workspace)
	}
	if len(cfg.Projects) != 3 {
		t.Fatalf("want 3 projects, got %d", len(cfg.Projects))
	}
	if !filepath.IsAbs(cfg.Projects[0].Path) {
		t.Errorf("paths must be absolute, got %q", cfg.Projects[0].Path)
	}
	if !cfg.Projects[2].Optional {
		t.Errorf("optional flag must propagate")
	}
	if cfg.Projects[0].Name != "keycloak" {
		t.Errorf("Name from basename, got %q", cfg.Projects[0].Name)
	}
}

// startOrder must reorder the projects list. Subs not listed get appended in
// their original order.
func TestLoadMetaConfig_StartOrderReorders(t *testing.T) {
	dir := t.TempDir()
	path := writeMeta(t, dir, `
kind: meta
projects:
  - path: a
  - path: b
  - path: c
startOrder:
  - c
  - a
`)
	cfg, _, err := LoadMetaConfig(path)
	if err != nil {
		t.Fatalf("LoadMetaConfig: %v", err)
	}
	got := []string{cfg.Projects[0].Name, cfg.Projects[1].Name, cfg.Projects[2].Name}
	want := []string{"c", "a", "b"}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("order = %v, want %v", got, want)
			return
		}
	}
}

// kind: meta with no projects → loud error so the user notices the typo.
func TestLoadMetaConfig_RequiresProjects(t *testing.T) {
	dir := t.TempDir()
	path := writeMeta(t, dir, "kind: meta\n")
	_, isMeta, err := LoadMetaConfig(path)
	if !isMeta {
		t.Fatal("kind: meta must mark file as meta even on validation error")
	}
	if err == nil || !strings.Contains(err.Error(), "projects") {
		t.Errorf("expected error about missing projects, got %v", err)
	}
}

// startOrder entry not matching any project: loud error.
func TestLoadMetaConfig_StartOrderMustMatch(t *testing.T) {
	dir := t.TempDir()
	path := writeMeta(t, dir, `
kind: meta
projects:
  - path: a
startOrder:
  - typo
`)
	_, _, err := LoadMetaConfig(path)
	if err == nil || !strings.Contains(err.Error(), "typo") {
		t.Errorf("expected error referencing the typo, got %v", err)
	}
}

// projects[*].path required.
func TestLoadMetaConfig_PathRequired(t *testing.T) {
	dir := t.TempDir()
	path := writeMeta(t, dir, `
kind: meta
projects:
  - optional: true
`)
	_, _, err := LoadMetaConfig(path)
	if err == nil || !strings.Contains(err.Error(), "path") {
		t.Errorf("expected error about missing path, got %v", err)
	}
}

// router.project resolves to an absolute MetaProject. Existence of the
// target directory is intentionally NOT checked at parse time (matches
// the sibling-dep contract, ADR-008).
func TestLoadMetaConfig_RouterResolves(t *testing.T) {
	dir := t.TempDir()
	path := writeMeta(t, dir, `
kind: meta
workspace: hypixo
router:
  project: ./gateway
projects:
  - path: ./api
  - path: ./gateway
`)
	cfg, _, err := LoadMetaConfig(path)
	if err != nil {
		t.Fatalf("LoadMetaConfig: %v", err)
	}
	if cfg.Router == nil {
		t.Fatal("Router is nil; expected resolved MetaProject")
	}
	if !filepath.IsAbs(cfg.Router.Path) {
		t.Errorf("Router.Path = %q, want absolute", cfg.Router.Path)
	}
	if cfg.Router.Name != "gateway" {
		t.Errorf("Router.Name = %q, want %q", cfg.Router.Name, "gateway")
	}
	// Router path overlapping with a projects: entry is allowed — both
	// refer to the same sub-project. The router upgrade is purely
	// lifecycle-level.
	if len(cfg.Projects) != 2 {
		t.Errorf("expected 2 projects (gateway permitted to appear under "+
			"projects: alongside router:), got %d", len(cfg.Projects))
	}
}

// router with empty project: → loud error.
func TestLoadMetaConfig_RouterRequiresProject(t *testing.T) {
	dir := t.TempDir()
	path := writeMeta(t, dir, `
kind: meta
router: {}
projects:
  - path: ./api
`)
	_, _, err := LoadMetaConfig(path)
	if err == nil || !strings.Contains(err.Error(), "router.project") {
		t.Errorf("expected error referencing router.project, got %v", err)
	}
}

// router absent → MetaConfig.Router stays nil. Guards against the loader
// accidentally synthesizing an empty MetaProject when the field is omitted.
func TestLoadMetaConfig_RouterAbsentStaysNil(t *testing.T) {
	dir := t.TempDir()
	path := writeMeta(t, dir, `
kind: meta
projects:
  - path: ./api
`)
	cfg, _, err := LoadMetaConfig(path)
	if err != nil {
		t.Fatalf("LoadMetaConfig: %v", err)
	}
	if cfg.Router != nil {
		t.Errorf("Router = %+v on config without router:, want nil", cfg.Router)
	}
}
