package proxy

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

const (
	defaultCertsDir = ".raioz/certs"
	certFileName    = "cert.pem"
	keyFileName     = "cert-key.pem"
)

// CertsDir returns the default certificate directory (~/.raioz/certs/).
func CertsDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, defaultCertsDir)
}

// CertsDirForDomain returns the per-domain certificate directory. Namespacing
// by domain prevents the historical bug where a cert minted for acme.localhost
// got silently reused when the user switched to hypixo.dev, which caused
// Caddy to fall back to ACME and hang on DNS challenges.
func CertsDirForDomain(domain string) string {
	base := CertsDir()
	if base == "" {
		return ""
	}
	return filepath.Join(base, sanitizeDomainForPath(domain))
}

// sanitizeDomainForPath turns a domain into a filesystem-safe directory name.
// Domains are already restricted to a safe subset of characters, but path
// separators (and parent-traversal) are paranoia-stripped just in case.
func sanitizeDomainForPath(domain string) string {
	if domain == "" {
		domain = "localhost"
	}
	safe := strings.ReplaceAll(domain, string(filepath.Separator), "_")
	safe = strings.ReplaceAll(safe, "..", "_")
	return safe
}

// EnsureCerts generates mkcert certificates for the given domain.
// Domain defaults to "localhost" if empty (certs cover *.localhost).
// For custom domains like "acme.localhost", covers *.acme.localhost.
// Returns the certs directory path, or empty string if mkcert is not available.
//
// extraSANs are exact route FQDNs (e.g. "conorbi.localhost") that get minted
// as their own SANs in addition to <domain> + *.<domain>. Browsers reject a
// wildcard match when the parent is a single-label name (mkcert itself warns
// "many browsers don't support second-level wildcards like *.localhost"), so
// an apex hostname under `domain: localhost` is only trusted if its exact
// FQDN is present. curl/OpenSSL accept the wildcard, which makes the symptom
// look CLI-fine but browser-insecure.
//
// Certificates are stored under ~/.raioz/certs/<domain>/ so switching
// projects (e.g. acme.localhost → hypixo.dev) never silently reuses a cert
// minted for a different SAN. On top of the directory namespacing, the
// existing cert is parsed and its SAN list is verified to cover <domain>,
// *.<domain>, AND every extraSAN before we accept it — so adding a new route
// regenerates the cert rather than serving one missing the new FQDN.
//
// ctx cancels in-flight mkcert subprocesses — protects against
// unattended macOS keychain prompts that would otherwise hang.
func EnsureCerts(ctx context.Context, domain string, extraSANs ...string) (string, error) {
	if domain == "" {
		domain = "localhost"
	}

	dir := CertsDirForDomain(domain)
	if dir == "" {
		return "", fmt.Errorf("could not determine home directory")
	}

	certPath := filepath.Join(dir, certFileName)
	keyPath := filepath.Join(dir, keyFileName)

	if fileExists(certPath) && fileExists(keyPath) &&
		certMatchesDomain(certPath, domain, extraSANs) {
		return dir, nil
	}

	// Check if mkcert is available
	if !commandExists("mkcert") {
		return "", nil // Not an error — proxy works without HTTPS
	}

	// Create certs directory
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create certs directory: %w", err)
	}

	// Install root CA (if not already done). Best-effort — mkcert is
	// idempotent and returns non-zero when the CA is already trusted.
	install := exec.CommandContext(ctx, "mkcert", "-install")
	_ = install.Run()

	// Generate the cert: domain + wildcard, plus any exact route FQDNs.
	names := certSANs(domain, extraSANs)
	args := append([]string{"-cert-file", certPath, "-key-file", keyPath}, names...)
	gen := exec.CommandContext(ctx, "mkcert", args...)
	if output, err := gen.CombinedOutput(); err != nil {
		return "", fmt.Errorf("mkcert failed: %w\n%s", err, string(output))
	}

	return dir, nil
}

// certSANs returns the full SAN list to mint: <domain>, *.<domain>, then any
// extras not already covered by those two, de-duplicated and order-stable.
func certSANs(domain string, extraSANs []string) []string {
	names := []string{domain, "*." + domain}
	seen := map[string]bool{domain: true, "*." + domain: true}
	for _, san := range extraSANs {
		if san == "" || seen[san] {
			continue
		}
		seen[san] = true
		names = append(names, san)
	}
	return names
}

// certMatchesDomain returns true when the PEM certificate at certPath carries
// `domain`, `*.domain`, AND every entry in extraSANs in its Subject Alternative
// Names. Anything else (unreadable file, corrupted PEM, cert minted for a
// different project, stale CN-only cert, or a cert missing a route's exact
// FQDN) returns false so the caller triggers regeneration.
func certMatchesDomain(certPath, domain string, extraSANs []string) bool {
	data, err := os.ReadFile(certPath)
	if err != nil {
		return false
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return false
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return false
	}
	wantExact := domain
	wantWild := "*." + domain
	var hasExact, hasWild bool
	for _, n := range cert.DNSNames {
		switch n {
		case wantExact:
			hasExact = true
		case wantWild:
			hasWild = true
		}
	}
	if !hasExact || !hasWild {
		return false
	}
	// Every requested route FQDN must be present too — a wildcard the
	// browser won't honor (single-label parent) doesn't count as coverage.
	for _, san := range extraSANs {
		if san == "" || san == wantExact || san == wantWild {
			continue
		}
		if !slices.Contains(cert.DNSNames, san) {
			return false
		}
	}
	return true
}

// HasMkcert returns true if mkcert is installed.
func HasMkcert() bool {
	return commandExists("mkcert")
}

// HasExistingCerts reports whether pre-generated certificates exist anywhere
// under the raioz certs directory. Used at `up` time as a coarse "can we
// proceed under tls: mkcert without the mkcert binary?" check — it doesn't
// verify domain-specific SANs because at call time we don't always have the
// domain in scope. EnsureCerts() does the per-domain validation and
// regeneration when the domain IS known.
//
// For backwards compatibility we still recognize the legacy flat layout
// (~/.raioz/certs/cert.pem) in addition to the new per-domain layout.
func HasExistingCerts() bool {
	dir := CertsDir()
	if dir == "" {
		return false
	}
	// Legacy flat layout.
	if fileExists(filepath.Join(dir, certFileName)) &&
		fileExists(filepath.Join(dir, keyFileName)) {
		return true
	}
	// New per-domain layout: any subdir with cert + key counts.
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		sub := filepath.Join(dir, e.Name())
		if fileExists(filepath.Join(sub, certFileName)) &&
			fileExists(filepath.Join(sub, keyFileName)) {
			return true
		}
	}
	return false
}

func commandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
