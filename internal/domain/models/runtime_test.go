package models

import "testing"

// The dispatcher picks a runner per runtime, and discovery picks the
// container-vs-host address by the same split. A runtime landing on the
// wrong side means either no runner or the wrong hostname injected.
func TestDetectResultIsDockerIsHost(t *testing.T) {
	docker := []Runtime{RuntimeCompose, RuntimeDockerfile, RuntimeImage}
	for _, rt := range docker {
		d := DetectResult{Runtime: rt}
		if !d.IsDocker() {
			t.Errorf("%s must be docker-run", rt)
		}
		if d.IsHost() {
			t.Errorf("%s must not be host-run", rt)
		}
	}

	for _, rt := range []Runtime{RuntimeNPM, RuntimeGo, RuntimeMake, RuntimeBun} {
		d := DetectResult{Runtime: rt}
		if d.IsDocker() {
			t.Errorf("%s must not be docker-run", rt)
		}
		if !d.IsHost() {
			t.Errorf("%s must be host-run", rt)
		}
	}

	// Unknown is the "detected nothing" sentinel: neither side, so
	// callers fall through to their own error instead of dispatching.
	unknown := DetectResult{Runtime: RuntimeUnknown}
	if unknown.IsDocker() || unknown.IsHost() {
		t.Error("RuntimeUnknown must belong to neither side")
	}
}

// AllRuntimes feeds the orchestrate registry's exhaustiveness test
// (ADR-019). A duplicate or the Unknown sentinel sneaking in would make
// that test assert the wrong thing.
func TestAllRuntimesIsCleanAndSplitsInTwo(t *testing.T) {
	all := AllRuntimes()
	seen := make(map[Runtime]bool, len(all))
	for _, rt := range all {
		if rt == RuntimeUnknown {
			t.Error("AllRuntimes must not include the Unknown sentinel")
		}
		if seen[rt] {
			t.Errorf("%s listed twice", rt)
		}
		seen[rt] = true

		d := DetectResult{Runtime: rt}
		if d.IsDocker() == d.IsHost() {
			t.Errorf("%s is neither or both docker-run and host-run", rt)
		}
	}
}
