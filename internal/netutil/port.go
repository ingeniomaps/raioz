// Package netutil holds host-side network utilities shared across raioz
// packages. Pure operations (port-bind probes, IP validation) live here
// so app/cli callers can reach them without importing infra adapters.
//
// New helpers added here MUST be free of docker / proxy / orchestrate
// dependencies — netutil is positioned below them in the import graph,
// not above.
package netutil

import (
	"fmt"
	"net"
	"strconv"
	"strings"
)

// CheckPortInUse reports whether a host port is currently in use by
// trying to bind a TCP listener to it. Accepts either a bare port
// number ("8080") or a "host:container" mapping ("8080:80"); only the
// host side is checked. Returns (true, nil) when the bind fails for
// any reason (port busy is the common case), (false, nil) on success.
//
// Format errors return an error so the caller can surface a clear
// message instead of treating a malformed port as "available".
func CheckPortInUse(port string) (bool, error) {
	parts := strings.Split(port, ":")
	if len(parts) == 0 || parts[0] == "" {
		return false, fmt.Errorf("invalid port format: %s", port)
	}
	hostPortStr := parts[0]
	hostPort, err := strconv.Atoi(hostPortStr)
	if err != nil {
		return false, fmt.Errorf("invalid port number: %s", hostPortStr)
	}

	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", hostPort))
	if err != nil {
		return true, nil
	}
	_ = ln.Close()
	return false, nil
}
