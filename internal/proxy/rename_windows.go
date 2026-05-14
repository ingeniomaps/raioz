//go:build windows

package proxy

import (
	"errors"
	"os"
	"syscall"
	"time"
)

// Windows-side errnos os.Rename can produce when another handle is open
// on either src or dst. Stable values from <winerror.h>.
const (
	winAccessDenied      syscall.Errno = 5  // ERROR_ACCESS_DENIED
	winSharingViolation  syscall.Errno = 32 // ERROR_SHARING_VIOLATION
)

// renameWithRetry replaces dst atomically. Unix's rename(2) makes this
// trivial; on Windows the antivirus / indexer / a concurrent reader can
// hold a handle on dst for tens of milliseconds and surface
// ERROR_SHARING_VIOLATION or ERROR_ACCESS_DENIED. Retry with a short
// backoff so the routes-persist write doesn't fail spuriously when a
// sibling goroutine just finished reading.
func renameWithRetry(src, dst string) error {
	const maxAttempts = 8
	delay := 5 * time.Millisecond
	var lastErr error
	for i := 0; i < maxAttempts; i++ {
		if err := os.Rename(src, dst); err == nil {
			return nil
		} else {
			lastErr = err
			if !errors.Is(err, winAccessDenied) && !errors.Is(err, winSharingViolation) {
				return err
			}
		}
		time.Sleep(delay)
		delay *= 2 // 5ms → 10 → 20 → 40 → 80 → 160 → 320 → 640 (~1.27s total)
	}
	return lastErr
}
