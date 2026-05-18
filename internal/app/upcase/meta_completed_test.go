package upcase

import (
	"testing"

	"raioz/internal/protocol"
)

func TestMetaAlreadyCompleted_UnsetEnv(t *testing.T) {
	t.Setenv(protocol.MetaCompletedProjects, "")
	if metaAlreadyCompleted("anything") {
		t.Error("empty env must always miss")
	}
}

func TestMetaAlreadyCompleted_EmptyProjectName(t *testing.T) {
	t.Setenv(protocol.MetaCompletedProjects, "keycloak,accounts")
	if metaAlreadyCompleted("") {
		t.Error("empty project name must always miss (defensive)")
	}
}

func TestMetaAlreadyCompleted_Hit(t *testing.T) {
	t.Setenv(protocol.MetaCompletedProjects, "keycloak,accounts,strategist")
	for _, name := range []string{"keycloak", "accounts", "strategist"} {
		if !metaAlreadyCompleted(name) {
			t.Errorf("expected hit on %q", name)
		}
	}
}

func TestMetaAlreadyCompleted_Miss(t *testing.T) {
	t.Setenv(protocol.MetaCompletedProjects, "keycloak,accounts")
	if metaAlreadyCompleted("strategist") {
		t.Error("strategist is not on the list, expected miss")
	}
}

// Whitespace tolerance: producer trims by convention but consumer
// must not break on stray spaces (e.g. operator pastes the env var
// for debugging with a space after the comma).
func TestMetaAlreadyCompleted_TrimsWhitespace(t *testing.T) {
	t.Setenv(protocol.MetaCompletedProjects, " keycloak ,  accounts , strategist  ")
	for _, name := range []string{"keycloak", "accounts", "strategist"} {
		if !metaAlreadyCompleted(name) {
			t.Errorf("expected hit on %q despite surrounding whitespace", name)
		}
	}
}

// Exact-match: substring is not a hit (no "keyclo" matching "keycloak").
func TestMetaAlreadyCompleted_ExactOnly(t *testing.T) {
	t.Setenv(protocol.MetaCompletedProjects, "keycloak,accounts")
	if metaAlreadyCompleted("keyclo") {
		t.Error("substring match not allowed")
	}
	if metaAlreadyCompleted("keycloak-admin") {
		t.Error("prefix match not allowed")
	}
}
