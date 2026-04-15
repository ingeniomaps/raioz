package proxy

import (
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

	if !certMatchesDomain(certPath, "hypixo.dev") {
		t.Error("expected cert to match hypixo.dev")
	}
}

func TestCertMatchesDomain_MissingWildcard(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "cert.pem")
	writeSelfSignedCert(t, certPath, []string{"hypixo.dev"}) // no wildcard

	if certMatchesDomain(certPath, "hypixo.dev") {
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

	if certMatchesDomain(certPath, "hypixo.dev") {
		t.Error("cross-domain cert reuse should be rejected (BUG-11)")
	}
}

func TestCertMatchesDomain_CorruptFile(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "cert.pem")
	_ = os.WriteFile(certPath, []byte("not a cert"), 0o644)

	if certMatchesDomain(certPath, "hypixo.dev") {
		t.Error("garbage file must not pass SAN check")
	}
}

func TestCertMatchesDomain_MissingFile(t *testing.T) {
	if certMatchesDomain(filepath.Join(t.TempDir(), "nope.pem"), "x") {
		t.Error("missing file must not pass")
	}
}
