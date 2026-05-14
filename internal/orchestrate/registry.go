package orchestrate

import (
	"fmt"

	"raioz/internal/domain/models"
)

// runnerSelector returns the runner instance held on a Dispatcher
// that should handle one specific runtime. The registry stores
// selectors (rather than runners directly) because Dispatcher owns
// runner state — every selector receives the live *Dispatcher and
// reaches into its fields.
type runnerSelector func(d *Dispatcher) runner

// runnerRegistry maps each runtime to the selector that resolves its
// runner. Populated at package-init time by runner files calling
// `register`; consumed by Dispatcher.selectRunner. The exhaustiveness
// test in `registry_test.go` guarantees every runtime in
// models.AllRuntimes() has an entry — adding a Runtime constant
// without registering a runner fails CI rather than silently breaking
// at dispatch time.
var runnerRegistry = map[models.Runtime]runnerSelector{}

// register associates runtime `rt` with `sel`. Duplicate registration
// is a programming error; we panic so it surfaces during package
// init rather than letting a later registration silently overwrite
// the first.
func register(rt models.Runtime, sel runnerSelector) {
	if _, exists := runnerRegistry[rt]; exists {
		panic(fmt.Sprintf("orchestrate: duplicate runner registration for runtime %q", rt))
	}
	runnerRegistry[rt] = sel
}
