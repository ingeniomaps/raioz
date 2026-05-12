package state

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDeferred_MarkClearIs(t *testing.T) {
	s := &LocalState{}

	if s.IsDeferred("postgres") {
		t.Error("fresh state should report no deferred deps")
	}

	s.MarkDeferred("postgres")
	if !s.IsDeferred("postgres") {
		t.Error("postgres should be deferred after Mark")
	}
	if got := len(s.DeferredToSibling); got != 1 {
		t.Errorf("len = %d, want 1", got)
	}

	// Idempotent: marking twice does not duplicate.
	s.MarkDeferred("postgres")
	if got := len(s.DeferredToSibling); got != 1 {
		t.Errorf("Mark twice: len = %d, want 1 (no duplicates)", got)
	}

	s.MarkDeferred("redis")
	if got := len(s.DeferredToSibling); got != 2 {
		t.Errorf("after second dep: len = %d, want 2", got)
	}

	s.ClearDeferred("postgres")
	if s.IsDeferred("postgres") {
		t.Error("postgres should not be deferred after Clear")
	}
	if !s.IsDeferred("redis") {
		t.Error("redis should still be deferred after clearing postgres")
	}

	// Clearing a name that's not in the list is a no-op, not an error.
	s.ClearDeferred("never-was")
	if got := len(s.DeferredToSibling); got != 1 {
		t.Errorf("Clear no-op: len = %d, want 1", got)
	}
}

func TestDeferred_RoundTripJSON(t *testing.T) {
	dir := t.TempDir()
	state := &LocalState{
		Project:           "accounts",
		DeferredToSibling: []string{"keycloak", "kafka"},
	}
	if err := SaveLocalState(dir, state); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := LoadLocalState(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got := strings.Join(loaded.DeferredToSibling, ","); got != "keycloak,kafka" {
		t.Errorf("DeferredToSibling = %q, want kafka,keycloak preserving order", got)
	}
}

// An empty deferred list must NOT serialize as `"deferredToSibling": []`
// — that would dirty diffs of every project that never used the
// feature. omitempty handles this; this test guards against accidental
// removal of the tag.
func TestDeferred_OmitemptyKeepsFileTidy(t *testing.T) {
	state := &LocalState{Project: "x"}

	data, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(data), "deferredToSibling") {
		t.Errorf("empty deferred list should be omitted, got: %s", data)
	}

	state.MarkDeferred("postgres")
	data, err = json.Marshal(state)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(data), `"deferredToSibling":["postgres"]`) {
		t.Errorf("non-empty deferred list should be persisted, got: %s", data)
	}
}
