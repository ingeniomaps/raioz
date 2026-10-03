package proxy

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"strconv"
	"time"

	"raioz/internal/logging"
)

// loopbackHost is where a published proxy listens unless `raioz up --host`
// says otherwise.
const loopbackHost = "127.0.0.1"

// proxyReadyTimeout bounds how long Start waits for Caddy to answer before
// giving up and returning anyway.
const proxyReadyTimeout = 10 * time.Second

// publishHost is the host address the proxy's 80/443 are bound on.
func (m *Manager) publishHost() string {
	if m.bindHost != "" {
		return m.bindHost
	}
	return loopbackHost
}

// listenHost is where a client reaches the proxy: the published host
// address, or the container's own IP when the proxy does not publish. Empty
// when neither is known.
func (m *Manager) listenHost(containerIP string) string {
	if m.publish {
		host := m.publishHost()
		if host == "0.0.0.0" || host == "::" {
			host = loopbackHost
		}
		return host
	}
	return containerIP
}

// proxyAnswers reports whether the proxy on host is serving: a completed
// TLS handshake on 443, or any HTTP response on 80 (the proxy without
// certificates serves plain HTTP only).
//
// An accepted TCP connection proves nothing here. With published ports the
// connection is accepted by Docker's own forwarder the moment the container
// exists, a second or two before Caddy inside it is listening.
//
// A package var so tests do not need a serving proxy.
var proxyAnswers = func(ctx context.Context, host string) bool {
	dialer := &net.Dialer{Timeout: 500 * time.Millisecond}
	// The probe only asks "is anything serving TLS here"; it sends nothing
	// and trusts nothing, so the certificate is not checked.
	tlsDialer := &tls.Dialer{
		NetDialer: dialer,
		Config:    &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // readiness probe, nothing is sent
	}
	if conn, err := tlsDialer.DialContext(ctx, "tcp", net.JoinHostPort(host, "443")); err == nil {
		_ = conn.Close()
		return true
	}

	client := &http.Client{
		Timeout:   500 * time.Millisecond,
		Transport: &http.Transport{DialContext: dialer.DialContext},
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	url := "http://" + net.JoinHostPort(host, "80") + "/"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return false
	}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	_ = resp.Body.Close()
	return true
}

// waitUntilListening holds Start until the proxy answers.
// `docker run -d` returns when the container exists, a second or two before
// Caddy has loaded its config: a caller that ran `raioz up && curl` hit a
// refused connection. Best-effort — on timeout Start still returns, and the
// proxy comes up when it comes up.
func (m *Manager) waitUntilListening(ctx context.Context, containerIP string) {
	host := m.listenHost(containerIP)
	if host == "" {
		return
	}
	deadline := time.Now().Add(proxyReadyTimeout)
	for time.Now().Before(deadline) {
		if proxyAnswers(ctx, host) {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(100 * time.Millisecond):
		}
	}
	logging.WarnWithContext(ctx, "Proxy did not answer within the wait",
		"host", host, "timeout", strconv.FormatFloat(proxyReadyTimeout.Seconds(), 'f', 0, 64)+"s")
}
