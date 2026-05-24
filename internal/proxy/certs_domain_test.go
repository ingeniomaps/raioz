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

// TestCertMatchesDomain_DifferentDomain reproduces BUG-11: a cert minted for
// acme.localhost must NOT be accepted as valid for hypixo.dev. Before the
// per-domain namespace + SAN check, this case silently passed.
func TestCertMatchesDomain_DifferentDomain(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "cert.pem")
	writeSelfSignedCert(t, certPath, []string{"acme.localhost", "*.acme.localhost"})

	if certMatchesDomain(certPath, "hypixo.dev", nil) {
		t.Error("cross-domain cert reuse should be rejected (BUG-11)")
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
