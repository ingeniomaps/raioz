package interfaces

import "strings"

// TLSMode is the vendor-neutral TLS provider value carried through
// the proxy port. ADR-032: the interface stops leaking
// Caddy/mkcert vocabulary by accepting only the typed values below.
// Adapters map TLSModeLocal → their own local CA tool (mkcert for
// Caddy today; could be different for a future Traefik adapter).
type TLSMode string

const (
	// TLSModeLocal — a local certificate authority signs the cert.
	// The Caddy adapter implements this via mkcert. Default when the
	// config is silent.
	TLSModeLocal TLSMode = "local"

	// TLSModeACME — automated certificate management via ACME
	// (Let's Encrypt and friends). Requires real DNS and reachable
	// host ports.
	TLSModeACME TLSMode = "acme"

	// TLSModeManual — the caller supplied a cert + key on disk.
	// raioz mounts what it finds and trusts the user.
	TLSModeManual TLSMode = "manual"
)

// ParseTLSMode normalizes user-supplied strings into the typed enum.
// Accepts:
//
//   - "" → TLSModeLocal (default)
//   - "local" / "acme" / "manual" → corresponding constants
//   - legacy aliases "mkcert" → TLSModeLocal, "letsencrypt" → TLSModeACME
//   - case-insensitive matching across all of the above
//
// Anything else returns ("", false). Callers MAY surface that as a
// warning; the canonical loader does, but tests of the helper can
// distinguish "unknown" from "valid but unusual."
func ParseTLSMode(s string) (TLSMode, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "":
		return TLSModeLocal, true
	case "local", "mkcert":
		return TLSModeLocal, true
	case "acme", "letsencrypt":
		return TLSModeACME, true
	case "manual":
		return TLSModeManual, true
	}
	return "", false
}
