package proxy

import (
	"context"
	"fmt"
	"net"
	"time"
)

// proxyHostPorts are the host ports a published proxy binds.
var proxyHostPorts = []int{80, 443}

// takenHostPorts returns the proxy host ports something else already holds.
func takenHostPorts() []int {
	var taken []int
	for _, p := range proxyHostPorts {
		if inUse, err := portCheckFunc(p); err == nil && inUse {
			taken = append(taken, p)
		}
	}
	return taken
}

// BusyHostPorts reports the host ports the proxy needs and cannot have, so
// `up` can refuse before it starts anything. Empty when the proxy does not
// publish, or when its container is already running (the ports are its own).
func (m *Manager) BusyHostPorts(ctx context.Context) []int {
	if !m.publish {
		return nil
	}
	if running, _ := m.isRunning(ctx, m.containerName()); running {
		return nil
	}
	return takenHostPorts()
}

// checkPortsAvailable reports whether the host ports the proxy needs are
// free before we try to create the container. Returns a descriptive error
// listing the conflicting port(s) so the user can act.
func (m *Manager) checkPortsAvailable() error {
	taken := takenHostPorts()
	if len(taken) == 0 {
		return nil
	}
	return fmt.Errorf(
		"proxy cannot start: host port(s) %v already in use; "+
			"stop the conflicting process, or set proxy.publish: false to "+
			"reach the proxy through its container IP", taken,
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
// any privilege — works as non-root against privileged ports too. isConnRefused
// is OS-specific so the Windows variant matches WSAECONNREFUSED too.
func probeTCPDial(host string, port int) (inUse, probed bool) {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", host, port), tcpProbeTimeout)
	if err == nil {
		_ = conn.Close()
		return true, true
	}
	if isConnRefused(err) {
		return false, true
	}
	return false, false
}

// probeTCPBind attempts to bind host:port. Returns (inUse, probed) where
// `probed` is false when we couldn't determine either way. isAddrInUse covers
// WSAEADDRINUSE on Windows in addition to EADDRINUSE.
func probeTCPBind(host string, port int) (inUse, probed bool) {
	ln, err := net.Listen("tcp", fmt.Sprintf("%s:%d", host, port))
	if err == nil {
		_ = ln.Close()
		return false, true
	}
	if isAddrInUse(err) {
		return true, true
	}
	return false, false
}
