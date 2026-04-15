package config

import "testing"

func TestApplyOverrides_ReturnsDepsUnchanged(t *testing.T) {
	deps := &Deps{
		Project: Project{Name: "test"},
		Services: map[string]Service{
			"api": {Source: SourceConfig{Kind: "local", Path: "./api"}},
		},
		Infra: map[string]InfraEntry{
			"postgres": {Inline: &Infra{Image: "postgres:16"}},
		},
	}

	result, overrides, err := ApplyOverrides(deps)
	if err != nil {
		t.Fatalf("ApplyOverrides() error = %v", err)
	}
	if result != deps {
		t.Error("ApplyOverrides() should return the same deps pointer")
	}
	if overrides != nil {
		t.Errorf("ApplyOverrides() overrides = %v, want nil", overrides)
	}
}

func TestApplyOverrides_NilDeps(t *testing.T) {
	result, overrides, err := ApplyOverrides(nil)
	if err != nil {
		t.Fatalf("ApplyOverrides() error = %v", err)
	}
	if result != nil {
		t.Error("ApplyOverrides(nil) should return nil")
	}
	if overrides != nil {
		t.Errorf("ApplyOverrides(nil) overrides = %v, want nil", overrides)
	}
}
