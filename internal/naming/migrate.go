package naming

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// Dropped in legacy dirs after migration so subsequent startups
// skip them without rescanning.
const migratedMarker = ".raioz-migrated-to-xdg"

// migrationFailedBreadcrumb names the file dropped in the legacy
// dir when copyTree fails. Issue 073: the user inspecting the
// legacy dir later finds an explicit "migration failed at X with
// error Y" note instead of silence.
const migrationFailedBreadcrumb = ".raioz-migration-failed"

// MigrationSkippedPrefix marks notes whose meaning is "we tried
// and couldn't" rather than "we copied successfully". Callers
// (CLI root) branch on this prefix to escalate skipped notes to
// PrintWarning while success notes go to logging.Info.
const MigrationSkippedPrefix = "migration from "

// MigrateLegacyStateDirs copies legacy state dirs into RaiozStateDir()
// once on first run; ADR-022. Best-effort — a populated destination
// short-circuits, and existing files at the destination are never
// overwritten ("new location wins").
func MigrateLegacyStateDirs() ([]string, error) {
	dst := RaiozStateDir()
	if hasContent(dst) {
		return nil, nil
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return nil, fmt.Errorf("create state dir %s: %w", dst, err)
	}
	var notes []string
	for _, src := range LegacyStateDirs() {
		if src == dst {
			continue
		}
		info, err := os.Stat(src)
		if err != nil || !info.IsDir() {
			continue
		}
		if _, err := os.Stat(filepath.Join(src, migratedMarker)); err == nil {
			continue
		}
		if err := copyTree(src, dst); err != nil {
			breadcrumb := filepath.Join(src, migrationFailedBreadcrumb)
			_ = os.WriteFile(breadcrumb,
				fmt.Appendf(nil, "%s\nmigration to %s failed: %v\n",
					time.Now().Format(time.RFC3339), dst, err),
				0o644)
			notes = append(notes,
				fmt.Sprintf("%s%s skipped: %v", MigrationSkippedPrefix, src, err))
			continue
		}
		_ = os.WriteFile(
			filepath.Join(src, migratedMarker),
			[]byte("migrated to "+dst+"\n"),
			0o644,
		)
		notes = append(notes,
			fmt.Sprintf("migrated state from %s → %s", src, dst))
	}
	return notes, nil
}

// Missing path and empty dir both count as "no content" — the
// migrator should run in either case.
func hasContent(path string) bool {
	entries, err := os.ReadDir(path)
	if err != nil {
		return false
	}
	return len(entries) > 0
}

// New location wins: existing files at dst are left alone. Partial
// copies persist on error; the marker file is the only success signal.
func copyTree(src, dst string) error {
	err := filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return fmt.Errorf("rel %s: %w", path, err)
		}
		if rel == "." || rel == migratedMarker {
			return nil
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		if _, err := os.Stat(target); err == nil {
			// Destination wins.
			return nil
		}
		return copyFile(path, target, info.Mode())
	})
	if err != nil {
		return fmt.Errorf("walk %s: %w", src, err)
	}
	return nil
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src) //nolint:gosec // src is under the user's home/opt; not attacker-controlled.
	if err != nil {
		return fmt.Errorf("open %s: %w", src, err)
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(dst), err)
	}
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return fmt.Errorf("create %s: %w", dst, err)
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		_ = os.Remove(dst)
		return fmt.Errorf("copy %s -> %s: %w", src, dst, err)
	}
	return nil
}
