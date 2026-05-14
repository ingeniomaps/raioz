package config

import "testing"

// TestYAMLToDeps_PreUpScalar verifies the scalar form `preUp: make
// createdb` lands on deps.PreUpHook. ADR-024 contract: preUp is a
// new field independent of `pre:`, runs after infra/sibling-spawn.
func TestYAMLToDeps_PreUpScalar(t *testing.T) {
	cfg := &RaiozConfig{
		Project: "p",
		PreUp:   YAMLStringOrSlice{"make createdb"},
	}
	deps, err := YAMLToDeps(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deps.PreUpHook != "make createdb" {
		t.Errorf("PreUpHook = %q, want %q", deps.PreUpHook, "make createdb")
	}
	if deps.PreHook != "" {
		t.Errorf("preUp leaked into PreHook: %q", deps.PreHook)
	}
}

// TestYAMLToDeps_PreUpListJoinedWithAnd verifies the list form is
// joined with ` && ` so the executor (which splits on the same
// separator) sees each entry as a discrete command. Mirrors how
// `pre:` lists are bridged.
func TestYAMLToDeps_PreUpListJoinedWithAnd(t *testing.T) {
	cfg := &RaiozConfig{
		Project: "p",
		PreUp:   YAMLStringOrSlice{"make createdb", "make seed"},
	}
	deps, err := YAMLToDeps(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "make createdb && make seed"
	if deps.PreUpHook != want {
		t.Errorf("PreUpHook = %q, want %q", deps.PreUpHook, want)
	}
}

// TestYAMLToDeps_PreAndPreUpCoexist documents that the two hooks
// are independent fields and never overwrite each other.
func TestYAMLToDeps_PreAndPreUpCoexist(t *testing.T) {
	cfg := &RaiozConfig{
		Project: "p",
		Pre:     YAMLStringOrSlice{"./scripts/fetch-secrets.sh"},
		PreUp:   YAMLStringOrSlice{"make createdb"},
	}
	deps, err := YAMLToDeps(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deps.PreHook != "./scripts/fetch-secrets.sh" {
		t.Errorf("PreHook = %q, want fetch-secrets.sh", deps.PreHook)
	}
	if deps.PreUpHook != "make createdb" {
		t.Errorf("PreUpHook = %q, want make createdb", deps.PreUpHook)
	}
}
