package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestConfigCorpus locks the public raioz.yaml contract. Every fixture
// under testdata/configs/ must parse cleanly through LoadYAML — that is
// the same entry point real users hit. The fixtures themselves are
// curated to cover every documented combination of fields in
// docs/CONFIG_REFERENCE.md (workspace, proxy variants, sibling modes,
// service overrides, …). When a public schema field is added, a
// fixture must be added or updated; the check-config-fixtures script
// in scripts/ enforces that on CI.
//
// Failure mode this is designed to catch: a refactor changes
// UnmarshalYAML semantics or a struct tag in a backward-incompatible
// way and only one of many polymorphic shapes still parses. The
// per-fixture sub-test names make the broken combination obvious in
// the failure output.
func TestConfigCorpus(t *testing.T) {
	const corpusDir = "testdata/configs"

	entries, err := os.ReadDir(corpusDir)
	if err != nil {
		t.Fatalf("read %s: %v", corpusDir, err)
	}

	yamlCount := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			continue
		}
		yamlCount++

		t.Run(name, func(t *testing.T) {
			path := filepath.Join(corpusDir, name)

			// Meta-orchestrator fixtures go through LoadMetaConfig — the
			// same loader the CLI uses when `kind: meta`. They do not
			// have a `project:` (the meta shape isn't a project).
			if meta, isMeta, err := LoadMetaConfig(path); err == nil && isMeta {
				if meta == nil || len(meta.Projects) == 0 {
					t.Fatalf("LoadMetaConfig(%s): empty Projects list", name)
				}
				return
			}

			cfg, err := LoadYAML(path)
			if err != nil {
				t.Fatalf("LoadYAML(%s): %v", name, err)
			}
			if cfg == nil {
				t.Fatalf("LoadYAML(%s) returned nil config", name)
			}
			if cfg.Project == "" {
				t.Fatalf("LoadYAML(%s): empty Project — every fixture must "+
					"have a project name", name)
			}
		})
	}

	if yamlCount < 15 {
		t.Fatalf("corpus has %d fixtures; minimum is 15 per the corpus charter "+
			"(see internal/config/testdata/configs/README.md)",
			yamlCount)
	}
}
