package naming

import (
	"os"
	"path/filepath"
)

// RaiozStateDir returns the canonical directory for mutable runtime
// state (audit log, ignored services, workspaces, proxy routes).
// Resolution: RAIOZ_HOME → $XDG_STATE_HOME/raioz → ~/.local/state/raioz
// → os.TempDir()/raioz. Never returns empty. ADR-022.
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

// PortsLockFile returns the path to the global host-port allocation
// advisory lock. Held during `raioz up`'s port-allocation + spawn
// phase so two concurrent `up` invocations in different workspaces
// can't both probe-and-claim the same host port and then race on
// `docker run -p`.
func PortsLockFile() string {
	return filepath.Join(RaiozStateDir(), "ports.lock")
}

// RaiozConfigDir returns the directory for user-authored preferences.
// XDG separates "state" (rebuildable) from "config" (user-authored);
// callers that store the latter use this instead of RaiozStateDir.
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

// LegacyStateDirs are the locations earlier raioz versions wrote to.
// MigrateLegacyStateDirs walks this list once on upgrade.
func LegacyStateDirs() []string {
	out := []string{"/opt/raioz-proyecto"}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		out = append(out, filepath.Join(home, ".raioz"))
		out = append(out, filepath.Join(home, ".raioz-data"))
	}
	return out
}
