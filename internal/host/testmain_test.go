package host

import (
	"os"
	"testing"
)

// TestMain redirects raioz's state dir into a throwaway directory for the
// whole package. Host service logs live under naming.LogDir, which resolves
// through RaiozStateDir — without this, every test that starts a host
// service would drop a log inside the developer's real
// ~/.local/state/raioz/logs/<test project name>/ and leave it there.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "raioz-host-test-")
	if err != nil {
		panic("create temp state dir: " + err.Error())
	}
	if err := os.Setenv("RAIOZ_HOME", dir); err != nil {
		panic("set RAIOZ_HOME: " + err.Error())
	}
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}
