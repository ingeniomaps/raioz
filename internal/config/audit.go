package config

import (
	"fmt"
	"path/filepath"
)

// AuditYAMLStrict runs the ADR-036 hygiene rules (H1 secret scan, H2
// path safety) against the raioz.yaml at path and additionally treats
// H3 (image pinning) warnings as fatal errors. Used by
// `raioz up --audit-siblings` to preflight sibling / router yamls
// before they are spawned — H3 is normally a warning during regular
// load, but the opt-in flag elevates every gate to a hard error.
//
// Returns nil when the yaml passes all three gates.
func AuditYAMLStrict(path string) error {
	// LoadYAML runs H1 (ScanForSecrets) and H2 (validatePathSafety)
	// already; failures surface here as the first error.
	cfg, err := LoadYAML(path)
	if err != nil {
		return fmt.Errorf("%s: %w", filepath.Base(path), err)
	}

	if warnings := imagePinningWarnings(cfg); len(warnings) > 0 {
		return fmt.Errorf(
			"%s: image pinning gate (H3) failed under --audit-siblings: %s",
			filepath.Base(path), warnings[0],
		)
	}
	return nil
}
