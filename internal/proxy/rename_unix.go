//go:build !windows

package proxy

import (
	"fmt"
	"os"
)

// renameWithRetry is a thin alias on Unix — rename(2) is atomic and
// races between concurrent writers never fail. The Windows variant
// retries to absorb the equivalent ERROR_SHARING_VIOLATION there.
func renameWithRetry(src, dst string) error {
	if err := os.Rename(src, dst); err != nil {
		return fmt.Errorf("rename %s → %s: %w", src, dst, err)
	}
	return nil
}
