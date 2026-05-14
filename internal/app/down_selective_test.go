package app

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"testing"

	"raioz/internal/domain/models"
)

// declaredServiceNames / declaredInfraNames / hasService / hasInfra are
// trivial but they're the resolution surface for the selective-down
// "is this a known target?" check. Pinning them prevents a future
// refactor from silently dropping a kind.
func TestHasServiceAndInfra(t *testing.T) {
	deps := &models.Deps{
		Services: map[string]models.Service{"api": {}, "web": {}},
		Infra:    map[string]models.InfraEntry{"postgres": {}, "redis": {}},
	}

	if !hasService(deps, "api") || !hasService(deps, "web") {
		t.Errorf("hasService missed a declared service")
	}
	if hasService(deps, "postgres") {
		t.Errorf("hasService matched a dep")
	}
	if !hasInfra(deps, "postgres") || !hasInfra(deps, "redis") {
		t.Errorf("hasInfra missed a declared dep")
	}
	if hasInfra(deps, "api") {
		t.Errorf("hasInfra matched a service")
	}
}

// captureStdout runs fn while os.Stdout is redirected to a buffer;
// returns whatever was printed. Used to verify selective-down skip
// messages go through `output.PrintInfo` without making the helper
// itself testable as a return value.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = orig })

	done := make(chan struct{})
	var buf bytes.Buffer
	go func() {
		_, _ = io.Copy(&buf, r)
		close(done)
	}()

	fn()
	_ = w.Close()
	<-done
	return buf.String()
}

func TestStopSelectiveDep_ModeASkipped(t *testing.T) {
	deps := &models.Deps{
		Infra: map[string]models.InfraEntry{
			"keycloak": {Inline: &models.Infra{Project: "/abs/sibling"}},
		},
	}
	out := captureStdout(t, func() {
		stopSelectiveDep(context.Background(), deps, "consumer", "keycloak", nil)
	})
	for _, want := range []string{"sibling-owned", "/abs/sibling", "raioz down"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\nfull: %s", want, out)
		}
	}
}

func TestStopSelectiveDep_ModeBDeferredSkipped(t *testing.T) {
	deps := &models.Deps{
		Infra: map[string]models.InfraEntry{
			"keycloak": {Inline: &models.Infra{
				Image:          "keycloak",
				SiblingProject: "/abs/sibling",
			}},
		},
	}
	ls := &models.LocalState{DeferredToSibling: []string{"keycloak"}}
	out := captureStdout(t, func() {
		stopSelectiveDep(context.Background(), deps, "consumer", "keycloak", ls)
	})
	if !strings.Contains(out, "deferred to sibling") {
		t.Errorf("expected deferred-skip message, got %q", out)
	}
}

func TestStopSelectiveDep_RegularDepNotSkipped(t *testing.T) {
	// Just verify the early-return paths don't fire — a regular dep with
	// no Project/Sibling/deferred state must reach the actual teardown
	// code path. We don't run the docker call here; the absence of the
	// skip message is the assertion.
	deps := &models.Deps{
		Infra: map[string]models.InfraEntry{
			"redis": {Inline: &models.Infra{Image: "redis", Tag: "7"}},
		},
	}
	out := captureStdout(t, func() {
		stopSelectiveDep(context.Background(), deps, "consumer", "redis", nil)
	})
	if strings.Contains(out, "sibling-owned") || strings.Contains(out, "deferred to sibling") {
		t.Errorf("regular dep should not hit sibling skip paths, got %q", out)
	}
}

func TestDeclaredNamesIncludeEverything(t *testing.T) {
	deps := &models.Deps{
		Services: map[string]models.Service{"api": {}, "web": {}},
		Infra:    map[string]models.InfraEntry{"postgres": {}},
	}

	svcs := declaredServiceNames(deps)
	if len(svcs) != 2 {
		t.Errorf("declaredServiceNames len = %d, want 2", len(svcs))
	}
	deps2Joined := strings.Join(declaredInfraNames(deps), ",")
	if deps2Joined != "postgres" {
		t.Errorf("declaredInfraNames = %q, want postgres", deps2Joined)
	}
}
