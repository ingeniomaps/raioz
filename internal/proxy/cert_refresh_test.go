package proxy

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"raioz/internal/domain/interfaces"
)

// fakeDockerLoggingCalls writes a fake container runtime that appends each
// invocation's first arg to logPath and answers the two inspect formats Start
// relies on: the State.Status probe (isRunning / removeStaleContainer) and the
// /certs mount-source lookup (mountedCertSource). mountSrc is echoed for the
// latter so a test can point the running proxy at any cert dir.
func fakeDockerLoggingCalls(t *testing.T, dir, logPath, mountSrc string) string {
	t.Helper()
	body := `#!/bin/sh
echo "$1" >> ` + logPath + `
case "$1" in
  inspect)
    case "$*" in
      *State.Status*) echo running ;;
      *Mounts*) printf '%s' '` + mountSrc + `' ;;
    esac
    ;;
  *) exit 0 ;;
esac
exit 0
`
	return writeFakeBinary(t, dir, "fakedocker", body)
}

// seedCert writes a cert+key pair with the given SANs into dir and returns dir.
func seedCert(t *testing.T, dir string, sans []string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	writeSelfSignedCert(t, filepath.Join(dir, certFileName), sans)
	if err := os.WriteFile(filepath.Join(dir, keyFileName), []byte("k"), 0o644); err != nil {
		t.Fatalf("key: %v", err)
	}
	return dir
}

func TestMountedCertSource(t *testing.T) {
	dir := t.TempDir()

	ok := writeFakeBinary(t, dir, "ok", "#!/bin/sh\nprintf '/some/certs/dir\\n'\n")
	withFakeRuntime(t, ok)
	m := NewManager("")
	if got := m.mountedCertSource(context.Background(), "c"); got != "/some/certs/dir" {
		t.Errorf("mountedCertSource = %q, want /some/certs/dir", got)
	}

	bad := writeFakeBinary(t, dir, "bad", "#!/bin/sh\nexit 1\n")
	withFakeRuntime(t, bad)
	if got := m.mountedCertSource(context.Background(), "c"); got != "" {
		t.Errorf("mountedCertSource on inspect failure = %q, want empty", got)
	}
}

// A running proxy whose mounted cert is missing a route's FQDN must be
// recreated (rm + run), not merely reloaded — the read-only /certs mount
// can't be swapped by `caddy reload`.
func TestStart_AlreadyRunning_RecreatesWhenCertStale(t *testing.T) {
	home := withHome(t)
	withFreePortsForProxy(t)

	// Fresh per-domain cert covers the route FQDN, so EnsureCerts returns its
	// dir as m.certsDir without needing mkcert.
	freshDir := seedCert(t, filepath.Join(home, ".raioz", "certs", "localhost"),
		[]string{"localhost", "*.localhost", "api.localhost"})
	// The running container, however, mounts an older cert lacking api.localhost.
	staleDir := seedCert(t, filepath.Join(t.TempDir(), "stale"),
		[]string{"localhost", "*.localhost"})

	bin := t.TempDir()
	logPath := filepath.Join(bin, "calls.log")
	fake := fakeDockerLoggingCalls(t, bin, logPath, staleDir)
	withFakeRuntime(t, fake)
	t.Setenv("PATH", bin) // no mkcert on PATH

	m := NewManager("")
	m.projectName = "proj"
	m.tlsMode = "mkcert"
	m.AddRoute(context.Background(), interfaces.ProxyRoute{
		ServiceName: "api", Hostname: "api", Target: "api:3000",
	})

	if err := m.Start(context.Background(), "net"); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if m.certsDir != freshDir {
		t.Errorf("certsDir = %q, want %q", m.certsDir, freshDir)
	}
	calls := readFileString(t, logPath)
	if !strings.Contains(calls, "rm") || !strings.Contains(calls, "run") {
		t.Errorf("expected recreate (rm + run) on stale cert; calls:\n%s", calls)
	}
}

// A running proxy whose mounted cert already covers every needed SAN should
// only reload — no rm, no run — even when its dir differs from m.certsDir.
func TestStart_AlreadyRunning_ReloadsWhenCertCovers(t *testing.T) {
	home := withHome(t)
	withFreePortsForProxy(t)

	covering := []string{"localhost", "*.localhost", "api.localhost"}
	freshDir := seedCert(t, filepath.Join(home, ".raioz", "certs", "localhost"), covering)
	// Different dir, but its cert covers the same SANs → no recreate needed.
	mountedDir := seedCert(t, filepath.Join(t.TempDir(), "mounted"), covering)

	bin := t.TempDir()
	logPath := filepath.Join(bin, "calls.log")
	fake := fakeDockerLoggingCalls(t, bin, logPath, mountedDir)
	withFakeRuntime(t, fake)
	t.Setenv("PATH", bin)

	m := NewManager("")
	m.projectName = "proj"
	m.tlsMode = "mkcert"
	m.AddRoute(context.Background(), interfaces.ProxyRoute{
		ServiceName: "api", Hostname: "api", Target: "api:3000",
	})

	if err := m.Start(context.Background(), "net"); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if m.certsDir != freshDir {
		t.Errorf("certsDir = %q, want %q", m.certsDir, freshDir)
	}
	calls := readFileString(t, logPath)
	if strings.Contains(calls, "rm") || strings.Contains(calls, "run") {
		t.Errorf("covering cert must reload, not recreate; calls:\n%s", calls)
	}
	if !strings.Contains(calls, "exec") {
		t.Errorf("expected a reload (exec caddy reload); calls:\n%s", calls)
	}
}

func readFileString(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(b)
}
