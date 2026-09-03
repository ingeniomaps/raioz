package host

import (
	"context"
	"fmt"
	"net"
	"time"
)

// portProbeTimeout caps a single dial. `raioz status` probes one port per
// host service, so this is the worst case it can add per service — short
// enough to stay imperceptible on a local stack, long enough that a
// service busy serving another request still answers.
const portProbeTimeout = 250 * time.Millisecond

// probeAddrs are the loopback addresses tried, in order. Both stacks are
// probed because a service that binds `::` only (common with Node) is
// invisible from 127.0.0.1, and calling that service dead would be the
// same false negative this probe exists to avoid.
var probeAddrs = []string{"127.0.0.1", "[::1]"}

// IsPortListening reports whether something accepts TCP connections on the
// given loopback port.
//
// The probe DIALS rather than binds on purpose. Binding answers "is this
// port free", which is the opposite question, and under a non-root raioz it
// fails with EACCES on privileged ports regardless of who owns the socket.
// A completed dial means somebody is serving — which is what the caller
// wants to know, no matter which process it is.
//
// Anything but a clean connection counts as not listening: a refused
// connection is the ordinary "nobody bound it", and a port that neither
// accepts nor refuses within the timeout is not usable either way.
func IsPortListening(ctx context.Context, port int) bool {
	if port <= 0 {
		return false
	}
	dialer := net.Dialer{Timeout: portProbeTimeout}
	for _, addr := range probeAddrs {
		conn, err := dialer.DialContext(ctx, "tcp", fmt.Sprintf("%s:%d", addr, port))
		if err != nil {
			continue
		}
		_ = conn.Close()
		return true
	}
	return false
}
