// Package proxycase hosts use cases for `raioz proxy` and a
// preflight helper (ADR-016) the proxy CLI and `raioz
// doctor` can share to surface fixable issues before the proxy is
// touched.
package proxycase

import (
	"context"
	"net"
	"os/exec"
	"strconv"
)

// PreflightCheck describes the outcome of one pre-launch probe.
// Severity is a soft signal — the CLI may print warnings without
// failing — but Pass=false on a "required" check should block the
// caller from proceeding.
type PreflightCheck struct {
	Name     string
	Pass     bool
	Required bool
	Message  string
	Hint     string
}

// PreflightInput is what the preflight needs to know about the
// project's intended proxy configuration. Keeping the input as a
// small struct (rather than `interfaces.ProxyConfig`) lets the CLI
// and `raioz doctor` invoke the same checks without instantiating a
// full proxy lifecycle.
type PreflightInput struct {
	// Publish is whether the proxy will bind host ports 80/443. The
	// port-conflict check runs only when this is true.
	Publish bool
	// TLSMode is the resolved TLS provider ("mkcert" by default, or
	// "letsencrypt"). The mkcert check is required only when this is
	// "mkcert" or empty.
	TLSMode string
}

// RunPreflight executes every proxy preflight probe and returns the
// results in stable order. Probes are independent: one failing does
// not short-circuit the rest, so the caller can render the full
// picture.
func RunPreflight(_ context.Context, in PreflightInput) []PreflightCheck {
	out := []PreflightCheck{}
	out = append(out, checkMkcert(in))
	if in.Publish {
		out = append(out, checkHostPortFree(80))
		out = append(out, checkHostPortFree(443))
	}
	return out
}

// checkMkcert reports whether `mkcert` is on PATH. Required when the
// proxy is configured for mkcert TLS (the default in local dev); the
// hint points at the install URL.
func checkMkcert(in PreflightInput) PreflightCheck {
	required := in.TLSMode == "" || in.TLSMode == "mkcert"
	if _, err := exec.LookPath("mkcert"); err != nil {
		return PreflightCheck{
			Name:     "mkcert",
			Pass:     false,
			Required: required,
			Message:  "not on PATH",
			Hint:     "Install mkcert: https://github.com/FiloSottile/mkcert",
		}
	}
	return PreflightCheck{
		Name:     "mkcert",
		Pass:     true,
		Required: required,
		Message:  "installed",
	}
}

// checkHostPortFree probes whether tcp port `port` on localhost can
// be bound. We use a Listen-then-Close dance instead of asking the
// kernel for the FD because the goal is "would the proxy succeed?",
// not "is anyone listening?".
func checkHostPortFree(port int) PreflightCheck {
	addr := ":" + strconv.Itoa(port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return PreflightCheck{
			Name:     "host port " + strconv.Itoa(port),
			Pass:     false,
			Required: true,
			Message:  "already in use",
			Hint:     "Stop whatever is bound to " + addr + ", or set proxy.publish: false in raioz.yaml.",
		}
	}
	_ = ln.Close()
	return PreflightCheck{
		Name:     "host port " + strconv.Itoa(port),
		Pass:     true,
		Required: true,
		Message:  "available",
	}
}
