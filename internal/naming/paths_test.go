package naming

import (
	"os"
	"path/filepath"
	"testing"
)

// TestRaiozStateDir verifies the ADR-022 resolution order: RAIOZ_HOME
// wins outright, XDG_STATE_HOME takes precedence over the home
// fallback, and the home fallback lands on ~/.local/state/raioz.
func TestRaiozStateDir(t *testing.T) {
	t.Run("RAIOZ_HOME wins", func(t *testing.T) {
		t.Setenv("RAIOZ_HOME", "/explicit/override")
		t.Setenv("XDG_STATE_HOME", "/xdg/state")
		if got := RaiozStateDir(); got != "/explicit/override" {
			t.Errorf("RaiozStateDir() = %q, want /explicit/override", got)
		}
	})

	t.Run("XDG_STATE_HOME used when RAIOZ_HOME unset", func(t *testing.T) {
		t.Setenv("RAIOZ_HOME", "")
		t.Setenv("XDG_STATE_HOME", "/some/xdg")
		want := filepath.Join("/some/xdg", "raioz")
		if got := RaiozStateDir(); got != want {
			t.Errorf("RaiozStateDir() = %q, want %q", got, want)
		}
	})

	t.Run("home fallback when nothing set", func(t *testing.T) {
		t.Setenv("RAIOZ_HOME", "")
		t.Setenv("XDG_STATE_HOME", "")
		home, err := os.UserHomeDir()
		if err != nil || home == "" {
			t.Skip("UserHomeDir() unavailable on this platform")
		}
		want := filepath.Join(home, ".local", "state", "raioz")
		if got := RaiozStateDir(); got != want {
			t.Errorf("RaiozStateDir() = %q, want %q", got, want)
		}
	})
}

// TestRaiozConfigDir mirrors TestRaiozStateDir but for the
// configuration sibling (XDG_CONFIG_HOME / ~/.config/raioz).
func TestRaiozConfigDir(t *testing.T) {
	t.Run("RAIOZ_HOME wins", func(t *testing.T) {
		t.Setenv("RAIOZ_HOME", "/explicit/override")
		t.Setenv("XDG_CONFIG_HOME", "/xdg/config")
		if got := RaiozConfigDir(); got != "/explicit/override" {
			t.Errorf("RaiozConfigDir() = %q, want /explicit/override", got)
		}
	})

	t.Run("XDG_CONFIG_HOME used when RAIOZ_HOME unset", func(t *testing.T) {
		t.Setenv("RAIOZ_HOME", "")
		t.Setenv("XDG_CONFIG_HOME", "/some/xdg")
		want := filepath.Join("/some/xdg", "raioz")
		if got := RaiozConfigDir(); got != want {
			t.Errorf("RaiozConfigDir() = %q, want %q", got, want)
		}
	})

	t.Run("home fallback when nothing set", func(t *testing.T) {
		t.Setenv("RAIOZ_HOME", "")
		t.Setenv("XDG_CONFIG_HOME", "")
		home, err := os.UserHomeDir()
		if err != nil || home == "" {
			t.Skip("UserHomeDir() unavailable on this platform")
		}
		want := filepath.Join(home, ".config", "raioz")
		if got := RaiozConfigDir(); got != want {
			t.Errorf("RaiozConfigDir() = %q, want %q", got, want)
		}
	})
}

// TestLegacyStateDirs documents the audited legacy roots. The list is
// load-bearing for the migrator — losing an entry here silently
// strands user data on upgrade.
func TestLegacyStateDirs(t *testing.T) {
	got := LegacyStateDirs()
	if len(got) == 0 {
		t.Fatal("LegacyStateDirs() returned no entries")
	}
	if got[0] != "/opt/raioz-proyecto" {
		t.Errorf("first legacy dir = %q, want /opt/raioz-proyecto", got[0])
	}
}

// TestMigrateLegacyStateDirs walks the happy path: a populated legacy
// dir is mirrored into RaiozStateDir() and a marker file is left so a
// second run is a no-op. New-location-wins semantics are exercised by
// pre-seeding the destination with a colliding file.
func TestMigrateLegacyStateDirs(t *testing.T) {
	tmp := t.TempDir()
	legacy := filepath.Join(tmp, ".raioz")
	if err := os.MkdirAll(filepath.Join(legacy, "sub"), 0o755); err != nil {
		t.Fatalf("seed legacy dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "audit.log"), []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("seed audit.log: %v", err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "sub", "ignore.json"), []byte("[]"), 0o644); err != nil {
		t.Fatalf("seed ignore.json: %v", err)
	}

	// Force LegacyStateDirs() to return our tempdir entry by
	// pretending tmp is the user's home directory.
	t.Setenv("HOME", tmp)

	// Destination root: a fresh XDG_STATE_HOME inside tmp.
	xdg := filepath.Join(tmp, "xdg-state")
	t.Setenv("XDG_STATE_HOME", xdg)
	t.Setenv("RAIOZ_HOME", "")

	notes, err := MigrateLegacyStateDirs()
	if err != nil {
		t.Fatalf("MigrateLegacyStateDirs(): %v", err)
	}
	if len(notes) == 0 {
		t.Fatal("expected at least one migration note")
	}

	dst := filepath.Join(xdg, "raioz")
	if _, err := os.Stat(filepath.Join(dst, "audit.log")); err != nil {
		t.Errorf("audit.log not migrated: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dst, "sub", "ignore.json")); err != nil {
		t.Errorf("sub/ignore.json not migrated: %v", err)
	}
	if _, err := os.Stat(filepath.Join(legacy, ".raioz-migrated-to-xdg")); err != nil {
		t.Errorf("breadcrumb marker missing in legacy dir: %v", err)
	}

	// Second run with a populated destination must be a no-op:
	// MigrateLegacyStateDirs returns nil notes when dst has content.
	notes2, err := MigrateLegacyStateDirs()
	if err != nil {
		t.Fatalf("second MigrateLegacyStateDirs(): %v", err)
	}
	if len(notes2) != 0 {
		t.Errorf("second migration emitted %d notes, want 0 (dst already populated)", len(notes2))
	}
}

// TestMigrateLegacyStateDirs_destinationWins seeds the destination
// first, then runs the migrator. The migrator must skip entirely
// because dst already has content — protecting against accidental
// overwrite after a successful first migration.
func TestMigrateLegacyStateDirs_destinationWins(t *testing.T) {
	tmp := t.TempDir()

	xdg := filepath.Join(tmp, "xdg-state")
	dst := filepath.Join(xdg, "raioz")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatalf("seed dst: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dst, "audit.log"), []byte("new\n"), 0o644); err != nil {
		t.Fatalf("seed dst audit.log: %v", err)
	}

	// Legacy dir with conflicting file.
	legacy := filepath.Join(tmp, ".raioz")
	if err := os.MkdirAll(legacy, 0o755); err != nil {
		t.Fatalf("seed legacy dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "audit.log"), []byte("old\n"), 0o644); err != nil {
		t.Fatalf("seed legacy audit.log: %v", err)
	}

	t.Setenv("HOME", tmp)
	t.Setenv("XDG_STATE_HOME", xdg)
	t.Setenv("RAIOZ_HOME", "")

	notes, err := MigrateLegacyStateDirs()
	if err != nil {
		t.Fatalf("MigrateLegacyStateDirs(): %v", err)
	}
	if len(notes) != 0 {
		t.Errorf("dst-wins path emitted %d notes, want 0", len(notes))
	}

	// Existing file must not have been overwritten.
	got, err := os.ReadFile(filepath.Join(dst, "audit.log"))
	if err != nil {
		t.Fatalf("read dst audit.log: %v", err)
	}
	if string(got) != "new\n" {
		t.Errorf("dst audit.log = %q, want unchanged 'new\\n'", string(got))
	}
}
