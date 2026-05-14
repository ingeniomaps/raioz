package config

import "testing"

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

// List form must join with " && " so the executor's split sees each
// entry as a discrete command.
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
