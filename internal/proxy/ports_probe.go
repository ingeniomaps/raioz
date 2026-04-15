package proxy

import (
	"errors"
	"fmt"
	"net"
	"syscall"
	"time"
)

// checkPortsAvailable reports whether the host ports the proxy needs are
// free before we try to create the container. Returns a descriptive error
// listing the conflicting port(s) so the user can act (stop the other
// process, configure SetBindHost, etc.).
func (m *Manager) checkPortsAvailable() error {
	ports := []int{80, 443}
	var taken []int
	for _, p := range ports {
		inUse, err := portCheckFunc(p)
		if err != nil {
			continue
		}
		if inUse {
			taken = append(taken, p)
		}
	}
	if len(taken) == 0 {
		return nil
	}
	return fmt.Errorf(
		"proxy cannot start: host port(s) %v already in use. "+
			"Stop the conflicting process, or configure the proxy to bind to a "+
			"different address (see proxy.bindHost).", taken,
	)
}

// isHostPortInUse reports whether a host TCP port is already bound. The
// probe is two-stage:
//
//  1. Try a TCP DIAL against 127.0.0.1:<port>. If something accepts the
//     connection, the port is in use — works regardless of who's serving
//     it. Connection-refused means nobody's listening → port is free.
//  2. As a secondary signal try to bind. The historical bind-only probe
//     misreported privileged ports as busy when raioz ran non-root
//     (EACCES is "we can't bind", not "someone else has it").
//
// Returns (false, nil) only when both probes are inconclusive — at that
// point we let the actual `docker run` surface the real error.
func isHostPortInUse(port int) (bool, error) {
	if inUse, probed := probeTCPDial("127.0.0.1", port); probed {
		return inUse, nil
	}
	if inUse, probed := probeTCPBind("", port); probed {
		return inUse, nil
	}
	if inUse, probed := probeTCPBind("127.0.0.1", port); probed {
		return inUse, nil
	}
	return false, nil
}

// tcpProbeTimeout caps the dial probe so a single port check never blocks
// the up flow for more than a fraction of a second.
const tcpProbeTimeout = 250 * time.Millisecond

// portCheckFunc is the package-level probe used by checkPortsAvailable.
// Declared as a variable so tests can stub it out.
var portCheckFunc = isHostPortInUse

// probeTCPDial tries to OPEN a TCP connection to host:port. Doesn't require
// any privilege — works as non-root against privileged ports too.
func probeTCPDial(host string, port int) (inUse, probed bool) {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", host, port), tcpProbeTimeout)
	if err == nil {
		_ = conn.Close()
		return true, true
	}
	if errors.Is(err, syscall.ECONNREFUSED) {
		return false, true
	}
	return false, false
}

// probeTCPBind attempts to bind host:port. Returns (inUse, probed) where
// `probed` is false when we couldn't determine either way.
func probeTCPBind(host string, port int) (inUse, probed bool) {
	ln, err := net.Listen("tcp", fmt.Sprintf("%s:%d", host, port))
	if err == nil {
		_ = ln.Close()
		return false, true
	}
	if errors.Is(err, syscall.EADDRINUSE) {
		return true, true
	}
	if errors.Is(err, syscall.EACCES) || errors.Is(err, syscall.EPERM) {
		return false, false
	}
	return false, false
}
