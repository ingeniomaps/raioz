package naming

import (
	"os"
	"path/filepath"
)

// RaiozStateDir returns the directory where raioz persists mutable
// runtime state: audit log, ignored-services file, workspaces tree,
// proxy routes.
//
// Resolution order:
//
//  1. `RAIOZ_HOME` env override — explicit user choice, takes
//     precedence over XDG and over any fallback.
//  2. `$XDG_STATE_HOME/raioz` — the standard Linux convention.
//  3. `~/.local/state/raioz` — XDG default when the var isn't set.
//  4. `os.TempDir()/raioz` — degraded last resort when even home
//     discovery fails. Never returns empty.
//
// ADR-022 (issue 042) consolidated the three duplicated copies of
// this logic into one helper. `RaiozConfigDir` is the sibling for
// user configuration.
func RaiozStateDir() string {
	if home := os.Getenv("RAIOZ_HOME"); home != "" {
		return home
	}
	if xdg := os.Getenv("XDG_STATE_HOME"); xdg != "" {
		return filepath.Join(xdg, "raioz")
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Join(home, ".local", "state", "raioz")
	}
	return filepath.Join(os.TempDir(), "raioz")
}

// RaiozConfigDir returns the directory where raioz stores user-level
// preferences (language, defaults, etc.). Distinct from
// `RaiozStateDir` because XDG separates "state" from "config" — state
// is "you can rebuild it from primary sources", config is "the user
// authored it".
//
// Resolution order:
//
//  1. `RAIOZ_HOME` env override.
//  2. `$XDG_CONFIG_HOME/raioz`.
//  3. `~/.config/raioz`.
//  4. `os.TempDir()/raioz` as the last resort.
func RaiozConfigDir() string {
	if home := os.Getenv("RAIOZ_HOME"); home != "" {
		return home
	}
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "raioz")
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Join(home, ".config", "raioz")
	}
	return filepath.Join(os.TempDir(), "raioz")
}

// LegacyStateDirs returns the paths previous raioz versions used for
// state, in priority order. Used by the first-run migrator in
// `internal/cli/wiring.go` to lift existing data into
// `RaiozStateDir()` on upgrade. Each entry is a candidate; the
// migrator checks for existence and contents before acting.
func LegacyStateDirs() []string {
	out := []string{"/opt/raioz-proyecto"}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		out = append(out, filepath.Join(home, ".raioz"))
		out = append(out, filepath.Join(home, ".raioz-data"))
	}
	return out
}
