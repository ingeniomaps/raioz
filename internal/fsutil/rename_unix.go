//go:build !windows

package fsutil

import (
	"fmt"
	"os"
)

// RenameWithRetry is a thin wrapper on Unix — rename(2) is atomic and
// races between concurrent writers never fail. The Windows variant
// retries to absorb ERROR_SHARING_VIOLATION / ERROR_ACCESS_DENIED.
func RenameWithRetry(src, dst string) error {
	if err := os.Rename(src, dst); err != nil {
		return fmt.Errorf("rename %s → %s: %w", src, dst, err)
	}
	return nil
}
