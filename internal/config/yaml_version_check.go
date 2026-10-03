package config

import (
	"strconv"
	"strings"

	"raioz/internal/i18n"
)

// schemaVersionWarnings returns advisory warnings about the schema
// version declared (or missing) in the config. ADR-031:
// the field is now a real gate at warning level. Cases:
//
//   - Missing — soft warning, "consider adding".
//   - Newer than current — loud warning ("fields ignored; update raioz").
//   - Older than current — loud warning ("run raioz migrate yaml").
//   - Malformed — loud warning naming the bad value.
//
// See docs/CONFIG_REFERENCE.md#versioning for the evolution policy.
func schemaVersionWarnings(cfg *RaiozConfig) []string {
	if cfg == nil {
		return nil
	}
	if cfg.Version == "" {
		return []string{i18n.T("warning.version_missing", CurrentSchemaVersion)}
	}
	cmp, ok := compareSchemaVersion(cfg.Version, CurrentSchemaVersion)
	if !ok {
		return []string{i18n.T("warning.version_malformed",
			cfg.Version, CurrentSchemaVersion, CurrentSchemaVersion)}
	}
	switch {
	case cmp == 0:
		return nil
	case cmp > 0:
		return []string{i18n.T("warning.version_newer", cfg.Version, CurrentSchemaVersion)}
	default:
		return []string{i18n.T("warning.version_older", cfg.Version, CurrentSchemaVersion)}
	}
}

// compareSchemaVersion parses both values as non-negative integers and
// returns -1 / 0 / +1 like strings.Compare. The boolean is false when
// either value isn't a recognized schema number — callers branch into
// a "malformed" warning. Strings like "1.0" or "v1" fail by design;
// the schema number is an integer and the doc says so.
func compareSchemaVersion(declared, current string) (int, bool) {
	d, err := strconv.Atoi(strings.TrimSpace(declared))
	if err != nil || d < 0 {
		return 0, false
	}
	c, err := strconv.Atoi(strings.TrimSpace(current))
	if err != nil || c < 0 {
		return 0, false
	}
	switch {
	case d < c:
		return -1, true
	case d > c:
		return 1, true
	}
	return 0, true
}
