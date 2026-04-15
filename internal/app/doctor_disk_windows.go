//go:build windows

package app

import "golang.org/x/sys/windows"

// getFreeDiskSpaceGB returns free disk space in GB for the current working
// directory's drive. Returns -1 when the probe fails.
func getFreeDiskSpaceGB() float64 {
	// Probe the current drive — mirrors the Unix behavior of checking "/".
	pathPtr, err := windows.UTF16PtrFromString(".")
	if err != nil {
		return -1
	}
	var freeBytesAvailable, totalBytes, totalFreeBytes uint64
	if err := windows.GetDiskFreeSpaceEx(
		pathPtr, &freeBytesAvailable, &totalBytes, &totalFreeBytes,
	); err != nil {
		return -1
	}
	return float64(freeBytesAvailable) / (1024 * 1024 * 1024)
}
