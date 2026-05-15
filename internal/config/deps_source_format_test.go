package config

import (
	"os"
	"path/filepath"
	"testing"

	"raioz/internal/domain/models"
	"raioz/internal/i18n"
)

// TestSourceFormat_StampedByJSONLoader pins ADR-039: LoadDeps marks
// every loaded struct with SourceFormatLegacyJSON. Adding a new
// loader path without stamping is what this test catches.
func TestSourceFormat_StampedByJSONLoader(t *testing.T) {
	i18n.Init("en")
	ResetJSONDeprecationWarningForTest()

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

	deps, _, err := LoadDeps(path)
	if err != nil {
		t.Fatalf("LoadDeps: %v", err)
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

// TestSourceFormat_StampedByYAMLBridge pins ADR-039 on the yaml
// bridge: any deps that came through the yaml loader land as
// SourceFormatYAML, independent of the public `version:` field.
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

// TestSourceFormat_PreservedByFilters pins ADR-006: every clone
// site copies SourceFormat. If a new filter forgets to copy it,
// the filtered struct silently looks like legacy-json (zero
// value of the type), which breaks isYAMLMode downstream.
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

// TestSourceFormat_AliasMatchesModels guards against drift between
// the config package re-export and the domain/models definition.
// A divergence would make the constants type-incompatible.
func TestSourceFormat_AliasMatchesModels(t *testing.T) {
	if SourceFormatLegacyJSON != models.SourceFormatLegacyJSON {
		t.Errorf("config.SourceFormatLegacyJSON diverged from models")
	}
	if SourceFormatYAML != models.SourceFormatYAML {
		t.Errorf("config.SourceFormatYAML diverged from models")
	}
}
