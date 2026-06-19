package proxy

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"raioz/internal/domain/interfaces"
)

// writeSelfSignedCert emits a minimal self-signed cert with the given DNS
// SANs. Good enough for SAN-matching tests without actually chaining to a CA.
func writeSelfSignedCert(t *testing.T, path string, dnsNames []string) {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: dnsNames[0]},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		DNSNames:     dnsNames,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("cert: %v", err)
	}
	var buf []byte
	buf = append(buf, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})...)
	if err := os.WriteFile(path, buf, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func TestCertsDirForDomain_Namespaced(t *testing.T) {
	t.Setenv("HOME", "/tmp/fake-home")
	acme := CertsDirForDomain("acme.localhost")
	hypixo := CertsDirForDomain("hypixo.dev")
	if acme == hypixo {
		t.Fatalf("distinct domains must map to distinct directories: %s", acme)
	}
}

func TestSanitizeDomainForPath_TraversalStripped(t *testing.T) {
	got := sanitizeDomainForPath("../../etc/passwd")
	if got == ".." || got == "../../etc/passwd" {
		t.Errorf("path traversal not neutralized: %q", got)
	}
}

func TestCertMatchesDomain_Exact(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "cert.pem")
	writeSelfSignedCert(t, certPath, []string{"hypixo.dev", "*.hypixo.dev"})

	if !certMatchesDomain(certPath, "hypixo.dev", nil) {
		t.Error("expected cert to match hypixo.dev")
	}
}

func TestCertMatchesDomain_MissingWildcard(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "cert.pem")
	writeSelfSignedCert(t, certPath, []string{"hypixo.dev"}) // no wildcard

	if certMatchesDomain(certPath, "hypixo.dev", nil) {
		t.Error("cert without *.hypixo.dev must not match")
	}
}

// TestCertMatchesDomain_DifferentDomain reproduces cross-domain cert reuse: a cert minted for
// acme.localhost must NOT be accepted as valid for hypixo.dev. Before the
// per-domain namespace + SAN check, this case silently passed.
func TestCertMatchesDomain_DifferentDomain(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "cert.pem")
	writeSelfSignedCert(t, certPath, []string{"acme.localhost", "*.acme.localhost"})

	if certMatchesDomain(certPath, "hypixo.dev", nil) {
		t.Error("cross-domain cert reuse should be rejected")
	}
}

func TestCertMatchesDomain_CorruptFile(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "cert.pem")
	_ = os.WriteFile(certPath, []byte("not a cert"), 0o644)

	if certMatchesDomain(certPath, "hypixo.dev", nil) {
		t.Error("garbage file must not pass SAN check")
	}
}

func TestCertMatchesDomain_MissingFile(t *testing.T) {
	if certMatchesDomain(filepath.Join(t.TempDir(), "nope.pem"), "x", nil) {
		t.Error("missing file must not pass")
	}
}

// A cert covering localhost + *.localhost must NOT be accepted when an apex
// route FQDN (conorbi.localhost) is required but absent — *.localhost is a
// browser-rejected wildcard, so raioz must regenerate with the exact SAN.
func TestCertMatchesDomain_MissingExtraSAN(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "cert.pem")
	writeSelfSignedCert(t, certPath, []string{"localhost", "*.localhost"})

	if certMatchesDomain(certPath, "localhost", []string{"conorbi.localhost"}) {
		t.Error("cert missing the exact apex SAN must not match")
	}
	if !certMatchesDomain(certPath, "localhost", nil) {
		t.Error("same cert with no extra SANs required must still match")
	}
}

func TestCertMatchesDomain_ExtraSANPresent(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "cert.pem")
	writeSelfSignedCert(t, certPath,
		[]string{"localhost", "*.localhost", "conorbi.localhost"})

	if !certMatchesDomain(certPath, "localhost", []string{"conorbi.localhost"}) {
		t.Error("cert carrying the exact apex SAN must match")
	}
}

func TestCertSANs_Dedup(t *testing.T) {
	// Extras already covered by domain / *.domain, plus a dup and a blank,
	// must collapse — domain + wildcard first, then the unique apex FQDN.
	got := certSANs("localhost", []string{
		"localhost", "*.localhost", "conorbi.localhost", "conorbi.localhost", "",
	})
	want := []string{"localhost", "*.localhost", "conorbi.localhost"}
	if len(got) != len(want) {
		t.Fatalf("certSANs = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("certSANs = %v, want %v", got, want)
		}
	}
}

// TestRouteSANs_MixedDomains_CoversSiblingDomain is the regression test for
// the shared-proxy mixed-domain TLS gap: the workspace mounts a SINGLE cert,
// minted for the domain of whichever project owns the proxy (here landing on
// "localhost"). A sibling on a different proxy.domain (identity on
// "conorbi.localhost") routes fine — Caddy matches by Host — but before the
// fix its FQDN never entered the cert, so the browser rejected TLS. routeSANs
// must fold in sibling-domain FQDNs (and that domain's apex + wildcard).
func TestRouteSANs_MixedDomains_CoversSiblingDomain(t *testing.T) {
	// landing fixes the shared proxy's own domain at "localhost".
	m := makeSharedManager(t, "conorbi", "landing")
	m.domain = "localhost"
	m.AddRoute(t.Context(), interfaces.ProxyRoute{
		ServiceName: "landing", Hostname: "conorbi", Target: "landing:3000",
	})
	if err := m.SaveProjectRoutes(); err != nil {
		t.Fatalf("save landing routes: %v", err)
	}

	// identity runs in the SAME workspace but on a different domain.
	identity := NewManager("")
	identity.workspaceName = "conorbi"
	identity.projectName = "identity"
	identity.domain = "conorbi.localhost"
	identity.tlsMode = "mkcert"
	identity.AddRoute(t.Context(), interfaces.ProxyRoute{
		ServiceName: "identity", Hostname: "identity", Target: "identity:8080",
	})
	if err := identity.SaveProjectRoutes(); err != nil {
		t.Fatalf("save identity routes: %v", err)
	}

	// The cert is minted from the proxy-owning manager (landing). Its SAN set
	// must cover identity's FQDN on the other domain.
	got := m.routeSANs()
	sanSet := make(map[string]bool, len(got))
	for _, s := range got {
		sanSet[s] = true
	}

	for _, want := range []string{
		"identity.conorbi.localhost", // the sibling-domain route FQDN (the bug)
		"conorbi.localhost",          // sibling apex (also landing's own route)
		"*.conorbi.localhost",        // wildcard for future sibling subdomains
	} {
		if !sanSet[want] {
			t.Errorf("routeSANs missing %q; got %v", want, got)
		}
	}
}

// TestRouteSANs_MixedDomains_CertAcceptsSiblingFQDN ties the whole chain
// together: routeSANs → certSANs → a minted cert → certMatchesDomain. A cert
// built from the mixed-domain SAN set must validate as covering the sibling's
// exact FQDN, which is precisely what the browser checks.
func TestRouteSANs_MixedDomains_CertAcceptsSiblingFQDN(t *testing.T) {
	m := makeSharedManager(t, "conorbi", "landing")
	m.domain = "localhost"
	m.AddRoute(t.Context(), interfaces.ProxyRoute{
		ServiceName: "landing", Hostname: "conorbi", Target: "landing:3000",
	})
	if err := m.SaveProjectRoutes(); err != nil {
		t.Fatalf("save landing routes: %v", err)
	}

	identity := NewManager("")
	identity.workspaceName = "conorbi"
	identity.projectName = "identity"
	identity.domain = "conorbi.localhost"
	identity.tlsMode = "mkcert"
	identity.AddRoute(t.Context(), interfaces.ProxyRoute{
		ServiceName: "identity", Hostname: "identity", Target: "identity:8080",
	})
	if err := identity.SaveProjectRoutes(); err != nil {
		t.Fatalf("save identity routes: %v", err)
	}

	extras := m.routeSANs()
	dnsNames := certSANs(m.domain, extras)

	dir := t.TempDir()
	certPath := filepath.Join(dir, "cert.pem")
	writeSelfSignedCert(t, certPath, dnsNames)

	// The exact sibling FQDN must be present as a SAN — the wildcard
	// *.localhost would NOT cover identity.conorbi.localhost (two labels deep).
	if !certMatchesDomain(certPath, "localhost", extras) {
		t.Fatalf("cert minted from mixed-domain SANs must validate; SANs=%v", dnsNames)
	}
	if !certMatchesDomain(certPath, "localhost", []string{"identity.conorbi.localhost"}) {
		t.Errorf("cert must explicitly cover identity.conorbi.localhost; SANs=%v", dnsNames)
	}
}

func TestRouteSANs_ApexAndSubdomain(t *testing.T) {
	m := NewManager("")
	m.domain = "localhost"
	m.AddRoute(context.Background(), interfaces.ProxyRoute{
		ServiceName: "conorbi", Hostname: "conorbi", Target: "host.docker.internal:8000",
	})
	m.AddRoute(context.Background(), interfaces.ProxyRoute{
		ServiceName: "api", Hostname: "api", Target: "api:3000",
		Aliases: []string{"www"},
	})

	got := m.routeSANs()
	want := map[string]bool{
		"conorbi.localhost": true,
		"api.localhost":     true,
		"www.localhost":     true,
	}
	if len(got) != len(want) {
		t.Fatalf("routeSANs = %v, want keys %v", got, want)
	}
	for _, san := range got {
		if !want[san] {
			t.Errorf("unexpected SAN %q in %v", san, got)
		}
	}
}
