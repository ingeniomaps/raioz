//go:build windows

package validate

import "golang.org/x/sys/windows"

func availableDiskSpaceBytes(path string) (uint64, error) {
	var freeBytesAvailable, totalBytes, totalFreeBytes uint64
	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	if err := windows.GetDiskFreeSpaceEx(
		pathPtr, &freeBytesAvailable, &totalBytes, &totalFreeBytes,
	); err != nil {
		return 0, err
	}
	return freeBytesAvailable, nil
}
