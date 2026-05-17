// Package fsutil holds filesystem helpers shared across raioz packages.
// Atomic writes + cross-platform rename live here so state, audit,
// proxy, and any future writer can share the same correctness
// guarantees instead of each duplicating CreateTemp + Rename logic.
package fsutil

import (
	"fmt"
	"os"
	"path/filepath"
)

// WriteFileAtomic writes data to path via a temp file in the same
// directory followed by RenameWithRetry. Either the old file survives
// intact or the new file replaces it — a SIGKILL mid-write never
// leaves a zero-byte or partial file (the failure mode that issue 034
// documented for direct os.WriteFile callers).
//
// The temp file is removed on any failure path so partial writes do
// not leak temp files in the target directory.
func WriteFileAtomic(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp in %s: %w", dir, err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op after successful rename
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write %s: %w", tmpName, err)
	}
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return fmt.Errorf("chmod %s: %w", tmpName, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close %s: %w", tmpName, err)
	}
	return RenameWithRetry(tmpName, path)
}
