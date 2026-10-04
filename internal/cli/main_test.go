package cli

import (
	"os"
	"testing"
)

// TestMain points the raioz state dirs at a throwaway directory. The
// commands under test resolve the ignore list, the audit log and the global
// state through them; without the redirect a test run rewrites the
// developer's real ones.
func TestMain(m *testing.M) {
	stateDir, err := os.MkdirTemp("", "raioz-cli-test-")
	if err != nil {
		panic("create temp state dir: " + err.Error())
	}
	for _, v := range []string{"RAIOZ_HOME", "XDG_STATE_HOME"} {
		if err := os.Setenv(v, stateDir); err != nil {
			panic("set " + v + ": " + err.Error())
		}
	}
	code := m.Run()
	_ = os.RemoveAll(stateDir)
	os.Exit(code)
}
