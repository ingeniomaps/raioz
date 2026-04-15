//go:build !windows

package validate

import (
	"fmt"
	"syscall"
)

func availableDiskSpaceBytes(path string) (uint64, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, fmt.Errorf("statfs %q: %w", path, err)
	}
	// Bsize is the filesystem block size, always positive.
	return stat.Bavail * uint64(stat.Bsize), nil // #nosec G115
}
