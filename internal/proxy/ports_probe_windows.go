//go:build windows

package proxy

import (
	"errors"
	"syscall"
)

// Winsock error numbers Go's syscall package does not name on Windows.
// Values from <winerror.h>; stable since Win NT 4 SP2.
const (
	winsockECONNREFUSED syscall.Errno = 10061 // WSAECONNREFUSED
	winsockEADDRINUSE   syscall.Errno = 10048 // WSAEADDRINUSE
)

// isConnRefused reports whether err is a connection-refused signal from
// Winsock. Both the portable alias (syscall.ECONNREFUSED) and the raw
// Winsock errno match — Go's net package can surface either depending
// on how the socket was opened.
func isConnRefused(err error) bool {
	if errors.Is(err, syscall.ECONNREFUSED) {
		return true
	}
	return errors.Is(err, winsockECONNREFUSED)
}

// isAddrInUse reports whether err is an address-already-in-use signal.
// On Windows the kernel returns WSAEADDRINUSE; Go's net package surfaces
// EADDRINUSE too for some bind paths.
func isAddrInUse(err error) bool {
	if errors.Is(err, syscall.EADDRINUSE) {
		return true
	}
	return errors.Is(err, winsockEADDRINUSE)
}
