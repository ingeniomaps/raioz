package app

import (
	"testing"

	"raioz/internal/naming"
)

// Guard: the bulk teardown decision must use the active-prefix shared-dep
// predicate. Sanity-checks the wiring assumption that a workspace makes
// deps shared.
func TestIsSharedDep_WorkspaceMakesShared(t *testing.T) {
	naming.SetPrefix("conorbi")
	t.Cleanup(func() { naming.SetPrefix("") })
	if !naming.IsSharedDep("") {
		t.Error("with a workspace prefix, an unnamed dep must be shared")
	}
}
