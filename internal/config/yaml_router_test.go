package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	raiozerrors "raioz/internal/errors"
)

// LoadYAML on a plain project config with router: resolves the path to
// absolute and stores it back on cfg.Router.Project. Existence of the
// target is not checked at parse time.
func TestLoadYAML_RouterResolves(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "raioz.yaml")
	body := `
project: app
router:
  project: ./gateway
services:
  api:
    path: ./api
`
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	cfg, err := LoadYAML(path)
	if err != nil {
		t.Fatalf("LoadYAML: %v", err)
	}
	if cfg.Router == nil {
		t.Fatal("cfg.Router is nil; expected populated YAMLRouter")
	}
	if !filepath.IsAbs(cfg.Router.Project) {
		t.Errorf("cfg.Router.Project = %q, want absolute", cfg.Router.Project)
	}
	if filepath.Base(cfg.Router.Project) != "gateway" {
		t.Errorf("cfg.Router.Project = %q, want basename gateway",
			cfg.Router.Project)
	}
}

// router: with an empty project: field → hard error from the project-shape
// loader. Mirrors the meta loader behavior.
func TestLoadYAML_RouterRequiresProject(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "raioz.yaml")
	body := `
project: app
router: {}
services:
  api:
    path: ./api
`
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, err := LoadYAML(path)
	if err == nil || !strings.Contains(err.Error(), "router.project") {
		t.Errorf("expected error referencing router.project, got %v", err)
	}
}

// Path-safety H2 must reject router.project pointing at /etc and friends.
// Sibling project paths follow the same blocklist-only rule by design.
func TestLoadYAML_RouterRejectsSystemDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "raioz.yaml")
	body := `
project: app
router:
  project: /etc/raioz-router
services:
  api:
    path: ./api
`
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, err := LoadYAML(path)
	if err == nil {
		t.Fatal("expected H2 path-safety error for router.project under /etc")
	}
	var rerr *raiozerrors.RaiozError
	if !errors.As(err, &rerr) {
		t.Fatalf("expected *RaiozError, got %T: %v", err, err)
	}
	if rerr.Code != raiozerrors.ErrCodeUnsafePath {
		t.Errorf("expected UNSAFE_PATH, got %s", rerr.Code)
	}
	if rerr.Context["field"] != "router.project" {
		t.Errorf("expected field=router.project, got %v",
			rerr.Context["field"])
	}
	if rerr.Context["system_dir"] != "/etc" {
		t.Errorf("expected system_dir=/etc, got %v",
			rerr.Context["system_dir"])
	}
}
