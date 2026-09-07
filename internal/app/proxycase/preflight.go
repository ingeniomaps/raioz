// Package proxycase hosts use cases for `raioz proxy` and a
// preflight helper (ADR-016) the proxy CLI and `raioz
// doctor` can share to surface fixable issues before the proxy is
// touched.
package proxycase

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
