package production

import (
	"os"
	"testing"

	"raioz/internal/i18n"
)

// TestMain loads the English catalog: the comparison report is built from
// it, and an uninitialized catalog renders every label as its key.
func TestMain(m *testing.M) {
	i18n.Init("en")
	os.Exit(m.Run())
}
