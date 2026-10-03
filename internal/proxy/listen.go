package proxy

import (
	"context"
	"net"
	"strconv"
	"time"

	"raioz/internal/logging"
)

// loopbackHost is where a published proxy listens unless `raioz up --host`
// says otherwise.
const loopbackHost = "127.0.0.1"

// proxyReadyTimeout bounds how long Start waits for Caddy to accept
// connections before giving up and returning anyway.
const proxyReadyTimeout = 10 * time.Second

// publishHost is the host address the proxy's 80/443 are bound on.
func (m *Manager) publishHost() string {
	if m.bindHost != "" {
		return m.bindHost
	}
	return loopbackHost
}

// listenAddress is where a client reaches the proxy's HTTPS port: the
// published host address, or the container's own IP when the proxy does not
// publish. Empty when neither is known.
func (m *Manager) listenAddress(containerIP string) string {
	if m.publish {
		host := m.publishHost()
		if host == "0.0.0.0" || host == "::" {
			host = loopbackHost
		}
		return net.JoinHostPort(host, "443")
	}
	if containerIP == "" {
		return ""
	}
	return net.JoinHostPort(containerIP, "443")
}

// dialProxy reports whether something accepts connections on addr. A
// package var so tests do not need a listening proxy.
var dialProxy = func(ctx context.Context, addr string) bool {
	conn, err := (&net.Dialer{Timeout: 500 * time.Millisecond}).DialContext(ctx, "tcp", addr)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// waitUntilListening holds Start until the proxy accepts connections.
// `docker run -d` returns when the container exists, a second or two before
// Caddy has loaded its config: a caller that ran `raioz up && curl` hit a
// refused connection. Best-effort — on timeout Start still returns, and the
// proxy comes up when it comes up.
func (m *Manager) waitUntilListening(ctx context.Context, containerIP string) {
	addr := m.listenAddress(containerIP)
	if addr == "" {
		return
	}
	deadline := time.Now().Add(proxyReadyTimeout)
	for time.Now().Before(deadline) {
		if dialProxy(ctx, addr) {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(100 * time.Millisecond):
		}
	}
	logging.WarnWithContext(ctx, "Proxy did not accept connections within the wait",
		"address", addr, "timeout", strconv.FormatFloat(proxyReadyTimeout.Seconds(), 'f', 0, 64)+"s")
}
