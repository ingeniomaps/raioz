package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	raiozerrors "raioz/internal/errors"
	"raioz/internal/i18n"
)

func writeMinimalJSON(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	content := `{
		"schemaVersion": "1.0",
		"project": {"name": "p", "network": "n"},
		"services": {},
		"infra": {},
		"env": {"useGlobal": true, "files": []}
	}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// ADR-038 v0.9: LoadDeps now hard-errors on any .raioz.json input. The
// previous warning-then-load behaviour from v0.7 / v0.8 is gone.
func TestLoadDeps_HardErrorsOnLegacyJSON(t *testing.T) {
	i18n.Init("en")

	dir := t.TempDir()
	path := writeMinimalJSON(t, dir, "cfg.json")

	deps, warnings, err := LoadDeps(path)
	if err == nil {
		t.Fatal("LoadDeps must hard-error on .raioz.json (ADR-038 v0.9)")
	}
	if deps != nil || warnings != nil {
		t.Errorf("hard-error path must return nil deps/warnings; got %v, %v", deps, warnings)
	}

	var re *raiozerrors.RaiozError
	if !errors.As(err, &re) {
		t.Fatalf("error must be a *RaiozError so callers can inspect Code; got %T", err)
	}
	if re.Code != raiozerrors.ErrCodeLegacyJSONFormat {
		t.Errorf("Code = %q, want %q", re.Code, raiozerrors.ErrCodeLegacyJSONFormat)
	}
	if !strings.Contains(re.Error(), "raioz migrate yaml") {
		t.Errorf("error message must name the migration command; got %q", re.Error())
	}
	if !strings.Contains(re.Suggestion, "raioz migrate yaml") {
		t.Errorf("suggestion must name the migration command; got %q", re.Suggestion)
	}
}

// The migration helper bypasses the gate so `raioz migrate yaml` can
// still read the legacy file. Without this seam, migration would be
// blocked by its own hard-error.
func TestLoadDepsForMigration_ParsesLegacyJSON(t *testing.T) {
	i18n.Init("en")

	dir := t.TempDir()
	path := writeMinimalJSON(t, dir, "cfg.json")

	deps, _, err := LoadDepsForMigration(path)
	if err != nil {
		t.Fatalf("LoadDepsForMigration must succeed for migration use; got %v", err)
	}
	if deps == nil || deps.Project.Name != "p" {
		t.Fatalf("parse should produce the deps; got %+v", deps)
	}
	if deps.SourceFormat != SourceFormatLegacyJSON {
		t.Errorf("SourceFormat = %q, want %q", deps.SourceFormat, SourceFormatLegacyJSON)
	}
}
