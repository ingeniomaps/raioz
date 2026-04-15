package proxy

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"raioz/internal/domain/interfaces"
	rt "raioz/internal/runtime"
)

// writeFakeBinary creates an executable shell script at path that prints
// `stdout` to stdout and exits with `exitCode`. Returns the path.
//
// The script is tailored to simulate container-runtime subcommands used by
// the proxy package: inspect (isRunning), run/stop/rm, cp, exec.
func writeFakeBinary(t *testing.T, dir, name, body string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake binary scripts are POSIX-only")
	}
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o755); err != nil {
		t.Fatalf("write fake binary: %v", err)
	}
	return p
}

// withFakeRuntime swaps the container runtime binary for the duration of a
// test via rt.SetBinary and restores it on cleanup.
func withFakeRuntime(t *testing.T, path string) {
	t.Helper()
	prev := rt.Binary()
	rt.SetBinary(path)
	t.Cleanup(func() {
		// SetBinary ignores empty, so reset through env-like path.
		rt.SetBinary(prev)
	})
}

// withHome points HOME/RAIOZ_HOME at a temp dir so naming.ProxyDir resolves
// to a writable path inside the test.
func withHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("RAIOZ_HOME", home)
	return home
}

// withFreePortsForProxy stubs the proxy's port availability probe so tests
// exercising Start() don't need real 80/443 to be free. Restores the real
// probe on cleanup.
func withFreePortsForProxy(t *testing.T) {
	t.Helper()
	prev := portCheckFunc
	portCheckFunc = func(int) (bool, error) { return false, nil }
	t.Cleanup(func() { portCheckFunc = prev })
}

func TestIsRunning_ReturnsFalse_WhenInspectFails(t *testing.T) {
	dir := t.TempDir()
	// Script exits non-zero: inspect fails → isRunning returns (false, nil).
	fake := writeFakeBinary(t, dir, "fakedocker",
		"#!/bin/sh\nexit 1\n")
	withFakeRuntime(t, fake)

	m := NewManager("")
	m.SetProjectName("proj")
	running, err := m.isRunning(context.Background(), "raioz-proxy-proj")
	if err != nil {
		t.Fatalf("isRunning returned err: %v", err)
	}
	if running {
		t.Error("expected running=false when inspect fails")
	}
}

func TestIsRunning_ReturnsTrue_WhenStatusIsRunning(t *testing.T) {
	dir := t.TempDir()
	fake := writeFakeBinary(t, dir, "fakedocker",
		"#!/bin/sh\necho running\n")
	withFakeRuntime(t, fake)

	m := NewManager("")
	m.SetProjectName("proj")
	running, err := m.isRunning(context.Background(), "raioz-proxy-proj")
	if err != nil {
		t.Fatalf("isRunning: %v", err)
	}
	if !running {
		t.Error("expected running=true")
	}
}

func TestIsRunning_ReturnsFalse_WhenStatusIsNotRunning(t *testing.T) {
	dir := t.TempDir()
	fake := writeFakeBinary(t, dir, "fakedocker",
		"#!/bin/sh\necho exited\n")
	withFakeRuntime(t, fake)

	m := NewManager("")
	m.SetProjectName("proj")
	running, err := m.isRunning(context.Background(), "raioz-proxy-proj")
	if err != nil {
		t.Fatalf("isRunning: %v", err)
	}
	if running {
		t.Error("expected running=false for non-running status")
	}
}

func TestStatus_DelegatesToIsRunning(t *testing.T) {
	dir := t.TempDir()
	fake := writeFakeBinary(t, dir, "fakedocker",
		"#!/bin/sh\necho running\n")
	withFakeRuntime(t, fake)

	m := NewManager("")
	m.SetProjectName("proj")
	running, err := m.Status(context.Background())
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if !running {
		t.Error("expected Status=true")
	}
}

func TestStatus_ReturnsFalse_WhenInspectFails(t *testing.T) {
	dir := t.TempDir()
	fake := writeFakeBinary(t, dir, "fakedocker",
		"#!/bin/sh\nexit 42\n")
	withFakeRuntime(t, fake)

	m := NewManager("")
	m.SetProjectName("proj")
	running, err := m.Status(context.Background())
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if running {
		t.Error("expected Status=false")
	}
}

func TestStop_AlwaysReturnsNil(t *testing.T) {
	dir := t.TempDir()
	// Stop ignores exit codes — both success and failure paths return nil.
	for _, exit := range []int{0, 1} {
		t.Run("", func(t *testing.T) {
			body := "#!/bin/sh\nexit " + itoa(exit) + "\n"
			fake := writeFakeBinary(t, dir, "fakedocker", body)
			withFakeRuntime(t, fake)

			m := NewManager("")
			m.SetProjectName("proj")
			if err := m.Stop(context.Background()); err != nil {
				t.Errorf("Stop should swallow errors, got: %v", err)
			}
		})
	}
}

func TestReload_FailsWhenCaddyReloadFails(t *testing.T) {
	withHome(t)

	dir := t.TempDir()
	// `caddy reload` (the only docker invocation Reload makes now) fails.
	// Reload no longer does docker cp — the Caddyfile is bind-mounted, so
	// generateCaddyfile writing it on the host is enough to update the
	// container's view. We just need to ask Caddy to re-read.
	fake := writeFakeBinary(t, dir, "fakedocker",
		"#!/bin/sh\necho reload-error >&2\nexit 1\n")
	withFakeRuntime(t, fake)

	m := NewManager("")
	m.SetProjectName("proj")
	m.networkName = "net"
	m.AddRoute(context.Background(), interfaces.ProxyRoute{
		ServiceName: "api",
		Hostname:    "api",
		Target:      "api:3000",
	})

	err := m.Reload(context.Background())
	if err == nil {
		t.Fatal("expected error when caddy reload fails")
	}
	if !strings.Contains(err.Error(), "failed to reload proxy") {
		t.Errorf("expected reload error message, got: %v", err)
	}
}

func TestReload_FailsWhenExecReloadFails(t *testing.T) {
	withHome(t)

	dir := t.TempDir()
	// cp (first args: "cp ...") succeeds, everything else fails.
	// We dispatch on the first argument.
	fake := writeFakeBinary(t, dir, "fakedocker", `#!/bin/sh
case "$1" in
  cp) exit 0 ;;
  exec) echo reload-err >&2 ; exit 1 ;;
  *) exit 0 ;;
esac
`)
	withFakeRuntime(t, fake)

	m := NewManager("")
	m.SetProjectName("proj")
	m.networkName = "net"
	m.AddRoute(context.Background(), interfaces.ProxyRoute{
		ServiceName: "api",
		Hostname:    "api",
		Target:      "api:3000",
	})

	err := m.Reload(context.Background())
	if err == nil {
		t.Fatal("expected error when exec reload fails")
	}
	if !strings.Contains(err.Error(), "failed to reload proxy") {
		t.Errorf("expected reload error message, got: %v", err)
	}
}

func TestReload_SucceedsWhenBothSteps_Ok(t *testing.T) {
	withHome(t)

	dir := t.TempDir()
	fake := writeFakeBinary(t, dir, "fakedocker",
		"#!/bin/sh\nexit 0\n")
	withFakeRuntime(t, fake)

	m := NewManager("")
	m.SetProjectName("proj")
	m.networkName = "net"
	m.AddRoute(context.Background(), interfaces.ProxyRoute{
		ServiceName: "api",
		Hostname:    "api",
		Target:      "api:3000",
	})

	if err := m.Reload(context.Background()); err != nil {
		t.Fatalf("Reload: %v", err)
	}
}

func TestStart_FailsWhenRunFails_NotAlreadyRunning(t *testing.T) {
	withHome(t)
	withFreePortsForProxy(t)

	dir := t.TempDir()
	// inspect (isRunning) → fail (not running), run → fail
	fake := writeFakeBinary(t, dir, "fakedocker", `#!/bin/sh
case "$1" in
  inspect) exit 1 ;;
  run) echo run-err >&2 ; exit 1 ;;
  *) exit 0 ;;
esac
`)
	withFakeRuntime(t, fake)

	// Ensure mkcert path returns empty (no mkcert), so Start reaches the
	// docker run codepath without triggering cert generation.
	t.Setenv("PATH", dir) // only our fake binary; no mkcert

	m := NewManager("")
	m.SetProjectName("proj")
	m.SetTLSMode("") // skip EnsureCerts entirely
	m.AddRoute(context.Background(), interfaces.ProxyRoute{
		ServiceName: "api",
		Hostname:    "api",
		Target:      "api:3000",
	})

	err := m.Start(context.Background(), "net")
	if err == nil {
		t.Fatal("expected error when docker run fails")
	}
	if !strings.Contains(err.Error(), "failed to start proxy") {
		t.Errorf("expected start error, got: %v", err)
	}
	// Network name should be recorded on the manager.
	if m.networkName != "net" {
		t.Errorf("networkName = %q, want %q", m.networkName, "net")
	}
}

func TestStart_SucceedsWhenRunOk_MkcertMode(t *testing.T) {
	withHome(t)
	withFreePortsForProxy(t)

	dir := t.TempDir()
	fake := writeFakeBinary(t, dir, "fakedocker", `#!/bin/sh
case "$1" in
  inspect) exit 1 ;;
  run) exit 0 ;;
  *) exit 0 ;;
esac
`)
	withFakeRuntime(t, fake)
	// Exclude mkcert from PATH so EnsureCerts returns "" without calling it.
	t.Setenv("PATH", dir)

	m := NewManager("")
	m.SetProjectName("proj")
	m.SetBindHost("0.0.0.0")
	m.AddRoute(context.Background(), interfaces.ProxyRoute{
		ServiceName: "api",
		Hostname:    "api",
		Target:      "api:3000",
	})

	if err := m.Start(context.Background(), "net"); err != nil {
		t.Fatalf("Start: %v", err)
	}
}

func TestStart_AlreadyRunning_TriggersReload(t *testing.T) {
	withHome(t)

	dir := t.TempDir()
	// inspect → running, cp → ok, exec → ok
	fake := writeFakeBinary(t, dir, "fakedocker", `#!/bin/sh
case "$1" in
  inspect) echo running ; exit 0 ;;
  *) exit 0 ;;
esac
`)
	withFakeRuntime(t, fake)
	t.Setenv("PATH", dir)

	m := NewManager("")
	m.SetProjectName("proj")
	m.SetTLSMode("letsencrypt") // avoid mkcert codepath
	m.AddRoute(context.Background(), interfaces.ProxyRoute{
		ServiceName: "api",
		Hostname:    "api",
		Target:      "api:3000",
	})

	if err := m.Start(context.Background(), "net"); err != nil {
		t.Fatalf("Start (already running → reload): %v", err)
	}
}

func TestStart_MkcertMode_WithPreExistingCerts(t *testing.T) {
	home := withHome(t)
	withFreePortsForProxy(t)
	// Pre-create a SAN-matching cert under the default-domain folder so
	// EnsureCerts returns the per-domain dir without invoking mkcert.
	certsDir := filepath.Join(home, ".raioz", "certs", "localhost")
	if err := os.MkdirAll(certsDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeSelfSignedCert(t, filepath.Join(certsDir, certFileName),
		[]string{"localhost", "*.localhost"})
	if err := os.WriteFile(filepath.Join(certsDir, keyFileName),
		[]byte("k"), 0o644); err != nil {
		t.Fatalf("key: %v", err)
	}

	dir := t.TempDir()
	fake := writeFakeBinary(t, dir, "fakedocker", `#!/bin/sh
case "$1" in
  inspect) exit 1 ;;
  run) exit 0 ;;
  *) exit 0 ;;
esac
`)
	withFakeRuntime(t, fake)
	t.Setenv("PATH", dir)

	m := NewManager("")
	m.SetProjectName("proj")
	m.SetTLSMode("mkcert")
	m.AddRoute(context.Background(), interfaces.ProxyRoute{
		ServiceName: "api",
		Hostname:    "api",
		Target:      "api:3000",
	})

	if err := m.Start(context.Background(), "net"); err != nil {
		t.Fatalf("Start: %v", err)
	}
	// certsDir on the manager should have been populated from EnsureCerts.
	if m.certsDir != certsDir {
		t.Errorf("certsDir = %q, want %q", m.certsDir, certsDir)
	}
}

func TestEnsureCerts_MkcertSuccess(t *testing.T) {
	home := withHome(t)
	// Provide a fake mkcert on PATH that creates the cert+key files on disk.
	// The -install invocation exits 0 (no file writes). The generate
	// invocation writes both files.
	bin := t.TempDir()
	writeFakeBinary(t, bin, "mkcert", `#!/bin/sh
# First arg is a flag: "-install" or "-cert-file".
if [ "$1" = "-install" ]; then
  exit 0
fi
# Expect: -cert-file CERT -key-file KEY domain wildcard
cert=""
key=""
while [ $# -gt 0 ]; do
  case "$1" in
    -cert-file) cert="$2" ; shift 2 ;;
    -key-file)  key="$2"  ; shift 2 ;;
    *) shift ;;
  esac
done
printf 'cert' > "$cert"
printf 'key'  > "$key"
exit 0
`)
	t.Setenv("PATH", bin)

	got, err := EnsureCerts("localhost")
	if err != nil {
		t.Fatalf("EnsureCerts: %v", err)
	}
	want := filepath.Join(home, ".raioz", "certs", "localhost")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	// Files should exist under the per-domain folder.
	if _, err := os.Stat(filepath.Join(want, certFileName)); err != nil {
		t.Errorf("cert file missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(want, keyFileName)); err != nil {
		t.Errorf("key file missing: %v", err)
	}
}

func TestEnsureCerts_MkcertFailure(t *testing.T) {
	withHome(t)
	// mkcert exists but fails at generation time → EnsureCerts must wrap err.
	bin := t.TempDir()
	writeFakeBinary(t, bin, "mkcert", `#!/bin/sh
if [ "$1" = "-install" ]; then
  exit 0
fi
echo "mkcert boom" >&2
exit 2
`)
	t.Setenv("PATH", bin)

	got, err := EnsureCerts("custom.local")
	if err == nil {
		t.Fatal("expected error when mkcert fails")
	}
	if got != "" {
		t.Errorf("expected empty dir on failure, got %q", got)
	}
	if !strings.Contains(err.Error(), "mkcert failed") {
		t.Errorf("expected mkcert failure message, got: %v", err)
	}
}

// itoa avoids pulling strconv into the fake-binary wiring tests.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
