package config

import (
	"os"
	"testing"
)

// TestMain gives the suite a throwaway home. The code under test resolves
// the raioz state dir through RAIOZ_HOME, the XDG base and finally HOME;
// a test that exercises those fallbacks would otherwise create directories
// in the developer's real ~/.local/state/raioz.
func TestMain(m *testing.M) {
	home, err := os.MkdirTemp("", "raioz-config-test-")
	if err != nil {
		panic("create temp home: " + err.Error())
	}
	for _, v := range []string{"HOME", "RAIOZ_HOME", "XDG_STATE_HOME"} {
		if err := os.Setenv(v, home); err != nil {
			panic("set " + v + ": " + err.Error())
		}
	}
	code := m.Run()
	_ = os.RemoveAll(home)
	os.Exit(code)
}
