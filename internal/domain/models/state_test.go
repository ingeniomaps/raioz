package models

import "testing"

// newLocalState mirrors what state.LoadLocalState hands callers: the maps
// are always initialized there, and the mutators below assume it.
func newLocalState() *LocalState {
	return &LocalState{
		DevOverrides: make(map[string]DevOverride),
		HostPIDs:     make(map[string]int),
	}
}

// `raioz dev <dep> <path>` swaps an image for local code and `--reset`
// puts it back. Down and status read the override to know which of the
// two a dep currently is.
func TestLocalStateDevOverrides(t *testing.T) {
	s := newLocalState()

	if s.IsDevOverridden("db") {
		t.Error("a fresh state must have no overrides")
	}
	if _, ok := s.GetDevOverride("db"); ok {
		t.Error("GetDevOverride reported an override that was never added")
	}

	s.AddDevOverride("db", "postgres:16", "../db")
	if !s.IsDevOverridden("db") {
		t.Error("IsDevOverridden() = false right after adding")
	}
	got, ok := s.GetDevOverride("db")
	if !ok {
		t.Fatal("GetDevOverride() missed the override just added")
	}
	if got.OriginalImage != "postgres:16" || got.LocalPath != "../db" {
		t.Errorf("override = %+v, want the original image and local path", got)
	}
	if got.PromotedAt.IsZero() {
		t.Error("PromotedAt must be stamped — `raioz status` shows it")
	}

	// Promoting again replaces rather than duplicates.
	s.AddDevOverride("db", "postgres:17", "../db2")
	if got, _ := s.GetDevOverride("db"); got.OriginalImage != "postgres:17" {
		t.Errorf("second promote must replace, got %+v", got)
	}

	s.RemoveDevOverride("db")
	if s.IsDevOverridden("db") {
		t.Error("IsDevOverridden() = true after --reset")
	}
	// Removing something absent is how --reset behaves on a clean dep.
	s.RemoveDevOverride("nope")
}

// A dep deferred to a sibling project (ADR-008 mode B) was never started
// here, so the matching down must skip it. MarkDeferred is called on
// every up, so repeating it must not grow the list.
func TestLocalStateDeferred(t *testing.T) {
	s := newLocalState()

	if s.IsDeferred("kafka") {
		t.Error("a fresh state defers nothing")
	}

	s.MarkDeferred("kafka")
	s.MarkDeferred("kafka")
	if len(s.DeferredToSibling) != 1 {
		t.Errorf("re-marking must be a no-op, got %v", s.DeferredToSibling)
	}
	if !s.IsDeferred("kafka") {
		t.Error("IsDeferred() = false right after marking")
	}

	s.MarkDeferred("redis")
	s.ClearDeferred("kafka")
	if s.IsDeferred("kafka") {
		t.Error("kafka should be gone after ClearDeferred")
	}
	if !s.IsDeferred("redis") {
		t.Error("ClearDeferred must not touch the other entries")
	}

	// Clearing an absent dep is what up does when the sibling was never
	// deferred in the first place.
	s.ClearDeferred("nope")
	if len(s.DeferredToSibling) != 1 {
		t.Errorf("clearing an absent dep changed the list: %v", s.DeferredToSibling)
	}
}
