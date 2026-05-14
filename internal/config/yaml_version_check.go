package config

import (
	"fmt"
	"strconv"
	"strings"
)

// schemaVersionWarnings returns advisory warnings about the schema
// version declared (or missing) in the config. Issue 054 / ADR-031:
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
		return []string{
			"no 'version:' field declared in raioz.yaml; add `version: \"" +
				CurrentSchemaVersion + "\"` to lock the schema your config " +
				"targets — see docs/CONFIG_REFERENCE.md#versioning",
		}
	}
	cmp, ok := compareSchemaVersion(cfg.Version, CurrentSchemaVersion)
	if !ok {
		return []string{fmt.Sprintf(
			"raioz.yaml declares version: %q which is not a recognized "+
				"schema number (expected an integer like %q). This binary "+
				"will load the config as if version: %q — see "+
				"docs/CONFIG_REFERENCE.md#versioning",
			cfg.Version, CurrentSchemaVersion, CurrentSchemaVersion,
		)}
	}
	switch {
	case cmp == 0:
		return nil
	case cmp > 0:
		return []string{fmt.Sprintf(
			"raioz.yaml declares version: %q but this binary supports "+
				"version: %q. Fields introduced in newer schema versions "+
				"will be ignored. Update raioz to a newer release.",
			cfg.Version, CurrentSchemaVersion,
		)}
	default:
		return []string{fmt.Sprintf(
			"raioz.yaml declares version: %q but this binary expects "+
				"version: %q. Field semantics may have changed across the "+
				"bump. Run `raioz migrate yaml` to update the file.",
			cfg.Version, CurrentSchemaVersion,
		)}
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
