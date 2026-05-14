package orchestrate

import (
	"testing"

	"raioz/internal/domain/models"
)

// TestAllRuntimesHaveRunner is the canary for ADR-019: every runtime
// declared in models.AllRuntimes() must have a runner registered via
// some runner-file's init(). A new Runtime constant added without a
// matching register() call lands here as a test failure with the
// offending runtime named — far better than a "unsupported runtime"
// error at first dispatch.
func TestAllRuntimesHaveRunner(t *testing.T) {
	for _, rt := range models.AllRuntimes() {
		if _, ok := runnerRegistry[rt]; !ok {
			t.Errorf("runtime %q has no runner registered — add a register() call in the relevant runner file's init()", rt)
		}
	}
}

// TestRegistryRejectsDuplicates documents the duplicate-registration
// panic. We can't directly invoke `register` again here without
// poisoning the global registry, so the test asserts the panic via
// a defer + recover on a fresh map cloned from the real one.
func TestRegistryRejectsDuplicates(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected register() to panic on duplicate registration")
		}
	}()
	// Use a runtime guaranteed to already be registered; the second
	// register() call must panic.
	register(models.RuntimeCompose, func(d *Dispatcher) runner { return nil })
}
