//go:build windows

package lock

import "golang.org/x/sys/windows"

// stillActiveExitCode is the value GetExitCodeProcess returns while a
// process is still running (Win32 STILL_ACTIVE constant).
const stillActiveExitCode = 259

func isProcessRunning(pid int) bool {
	if pid <= 0 {
		return false
	}
	h, err := windows.OpenProcess(
		windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid),
	)
	if err != nil {
		return false
	}
	defer windows.CloseHandle(h)
	var code uint32
	if err := windows.GetExitCodeProcess(h, &code); err != nil {
		return false
	}
	return code == stillActiveExitCode
}
