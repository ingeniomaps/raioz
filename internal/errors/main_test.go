package errors

import (
	"os"
	"testing"

	"raioz/internal/i18n"
)

// TestMain loads the English catalog: titles and suggestions come from it,
// and an uninitialized catalog renders them as their keys.
func TestMain(m *testing.M) {
	i18n.Init("en")
	os.Exit(m.Run())
}
