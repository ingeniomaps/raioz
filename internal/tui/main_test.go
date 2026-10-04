package tui

import (
	"os"
	"testing"

	"raioz/internal/i18n"
)

// TestMain loads the English catalog: the dashboard takes its status
// messages from it, and an uninitialized catalog renders them as keys.
func TestMain(m *testing.M) {
	i18n.Init("en")
	os.Exit(m.Run())
}
