package naming

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// migratedMarker is the breadcrumb left in a legacy directory after
// the contents have been copied into RaiozStateDir(). On the next
// startup the migrator sees the file and skips the legacy entry
// without rescanning.
const migratedMarker = ".raioz-migrated-to-xdg"

// MigrateLegacyStateDirs copies the contents of any legacy state
// directories (see LegacyStateDirs) into the current
// RaiozStateDir(), once, on first run. ADR-022 documents the policy.
//
// Behavior:
//
//   - Skip when RaiozStateDir() already has content. Two reasons to
//     skip: fresh install on a system without a legacy dir (nothing
//     to do), or an earlier raioz already migrated. Either way the
//     new location is authoritative.
//   - Skip a legacy entry that doesn't exist, or that already carries
//     the migratedMarker file.
//   - For each remaining legacy dir, copy every file/subdir under it
//     into RaiozStateDir() and write the marker. Existing files at
//     the destination are NOT overwritten — the new location wins.
//
// Returns a list of human-readable strings describing what was done,
// suitable for logging. Errors are returned but the migration is
// best-effort: a single failure does not abort the process.
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
			notes = append(notes,
				fmt.Sprintf("migration from %s skipped: %v", src, err))
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

// hasContent reports whether `path` is a directory with at least one
// entry. Empty directories and missing paths both count as "no
// content"; the migrator runs in either case.
func hasContent(path string) bool {
	entries, err := os.ReadDir(path)
	if err != nil {
		return false
	}
	return len(entries) > 0
}

// copyTree mirrors src into dst preserving subdirectory structure.
// Existing files at the destination are left alone — the rule the
// migrator advertises is "new location wins". Returns the first
// error encountered; partial copies persist (the marker file is the
// only signal of success).
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
