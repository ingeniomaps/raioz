package fsutil

import "os"

// IsTerminal reports whether f is an interactive terminal. The character
// device bit alone is not enough: /dev/null is a character device too, and
// a command run with stdin redirected from it must not be prompted.
func IsTerminal(f *os.File) bool {
	if f == nil {
		return false
	}
	fi, err := f.Stat()
	if err != nil || fi.Mode()&os.ModeCharDevice == 0 {
		return false
	}
	if null, err := os.Stat(os.DevNull); err == nil && os.SameFile(fi, null) {
		return false
	}
	return true
}
