package config

import (
	"os"
	"path/filepath"
	"testing"

	"raioz/internal/domain/models"
	"raioz/internal/i18n"
)

// Every JSON load stamps SourceFormatLegacyJSON. ADR-039.
// Uses LoadDepsForMigration because LoadDeps hard-errors on JSON
// since v0.9 (ADR-038); the migration helper is the only surface
// that still parses .raioz.json.
func TestSourceFormat_StampedByJSONLoader(t *testing.T) {
	i18n.Init("en")

	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.json")
	const content = `{
		"schemaVersion": "1.0",
		"project": {"name": "p", "network": "n"},
		"services": {},
		"infra": {},
		"env": {"useGlobal": true, "files": []}
	}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	deps, _, err := LoadDepsForMigration(path)
	if err != nil {
		t.Fatalf("LoadDepsForMigration: %v", err)
	}
	if deps.SourceFormat != SourceFormatLegacyJSON {
		t.Errorf("SourceFormat = %q, want %q",
			deps.SourceFormat, SourceFormatLegacyJSON)
	}
	if deps.SchemaVersion != "1.0" {
		t.Errorf("SchemaVersion should still be stamped legacy "+
			"%q for the dual-flow callers; got %q",
			"1.0", deps.SchemaVersion)
	}
}

// Every YAML load stamps SourceFormatYAML, independent of the public
// `version:` field. ADR-039.
func TestSourceFormat_StampedByYAMLBridge(t *testing.T) {
	cfg := &RaiozConfig{
		Version: "1",
		Project: "p",
	}
	deps, err := YAMLToDeps(cfg)
	if err != nil {
		t.Fatalf("YAMLToDeps: %v", err)
	}
	if deps.SourceFormat != SourceFormatYAML {
		t.Errorf("SourceFormat = %q, want %q",
			deps.SourceFormat, SourceFormatYAML)
	}
	if deps.SchemaVersion != "2.0" {
		t.Errorf("SchemaVersion = %q, want %q (legacy "+
			"discriminator must remain until v1.0)",
			deps.SchemaVersion, "2.0")
	}
}

// ADR-006: every clone site must copy SourceFormat. A missed copy
// makes the filtered struct look like legacy-json (zero value),
// which silently breaks isYAMLMode downstream.
func TestSourceFormat_PreservedByFilters(t *testing.T) {
	deps := &Deps{
		SchemaVersion: "2.0",
		SourceFormat:  SourceFormatYAML,
		Project:       Project{Name: "p"},
		Services:      map[string]Service{},
		Infra:         map[string]InfraEntry{},
	}

	t.Run("FilterByProfile", func(t *testing.T) {
		out := FilterByProfile(deps, "default")
		if out.SourceFormat != SourceFormatYAML {
			t.Fatalf("FilterByProfile dropped SourceFormat: got %q",
				out.SourceFormat)
		}
	})
	t.Run("FilterByProfiles", func(t *testing.T) {
		out := FilterByProfiles(deps, []string{"default"})
		if out.SourceFormat != SourceFormatYAML {
			t.Fatalf("FilterByProfiles dropped SourceFormat: got %q",
				out.SourceFormat)
		}
	})
	t.Run("FilterByFeatureFlags", func(t *testing.T) {
		out, _ := FilterByFeatureFlags(deps, "default", nil)
		if out.SourceFormat != SourceFormatYAML {
			t.Fatalf("FilterByFeatureFlags dropped SourceFormat: got %q",
				out.SourceFormat)
		}
	})
	t.Run("FilterIgnoredServices", func(t *testing.T) {
		out, _, err := FilterIgnoredServices(deps)
		if err != nil {
			t.Fatalf("FilterIgnoredServices: %v", err)
		}
		if out.SourceFormat != SourceFormatYAML {
			t.Fatalf("FilterIgnoredServices dropped SourceFormat: got %q",
				out.SourceFormat)
		}
	})
}

// Re-exports must equal the source — otherwise the constants
// silently diverge by type.
func TestSourceFormat_AliasMatchesModels(t *testing.T) {
	if SourceFormatLegacyJSON != models.SourceFormatLegacyJSON {
		t.Errorf("config.SourceFormatLegacyJSON diverged from models")
	}
	if SourceFormatYAML != models.SourceFormatYAML {
		t.Errorf("config.SourceFormatYAML diverged from models")
	}
}
