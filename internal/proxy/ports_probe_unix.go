//go:build !windows

package proxy

import (
	"errors"
	"syscall"
)

// isConnRefused reports whether err is a connection-refused signal from
// the kernel. On Unix this is plain ECONNREFUSED.
func isConnRefused(err error) bool {
	return errors.Is(err, syscall.ECONNREFUSED)
}

// isAddrInUse reports whether err is an address-already-in-use signal.
// On Unix the kernel returns EADDRINUSE; bind/listen wraps it.
func isAddrInUse(err error) bool {
	return errors.Is(err, syscall.EADDRINUSE)
}
