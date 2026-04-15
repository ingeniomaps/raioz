//go:build !windows

package app

import "syscall"

// getFreeDiskSpaceGB returns free disk space in GB for the root filesystem
func getFreeDiskSpaceGB() float64 {
	var stat syscall.Statfs_t
	if err := syscall.Statfs("/", &stat); err != nil {
		return -1
	}
	// Bsize is the filesystem block size, always positive.
	return float64(stat.Bavail*uint64(stat.Bsize)) / (1024 * 1024 * 1024) // #nosec G115
}
