package app

import (
	"strings"
	"testing"

	"raioz/internal/config"
)

func TestFilterSet_EmptyMeansNoFilter(t *testing.T) {
	if filterSet(nil) != nil {
		t.Errorf("nil filter should produce nil set")
	}
	if filterSet([]string{}) != nil {
		t.Errorf("empty slice should produce nil set")
	}
}

func TestInFilter_NilMatchesEverything(t *testing.T) {
	if !inFilter(nil, "any") {
		t.Errorf("nil filter must match every name")
	}
}

func TestInFilter_OnlyDeclared(t *testing.T) {
	want := filterSet([]string{"front", "redis"})
	if !inFilter(want, "front") {
		t.Errorf("front should match")
	}
	if inFilter(want, "back") {
		t.Errorf("back should NOT match")
	}
}

func TestCountMatching_SkipsHidden(t *testing.T) {
	infra := map[string]config.InfraEntry{
		"redis": {}, "postgres": {},
	}
	want := filterSet([]string{"redis"})
	if countMatching(infra, want) != 1 {
		t.Errorf("expected 1 visible dep, got %d", countMatching(infra, want))
	}
	if countMatching(infra, nil) != 2 {
		t.Errorf("nil filter should show all (got %d)", countMatching(infra, nil))
	}
}

func TestValidateStatusFilter_UnknownNameFails(t *testing.T) {
	proj := &YAMLProject{
		ProjectName: "demo",
		Deps: &config.Deps{
			Services: map[string]config.Service{"front": {}, "back": {}},
			Infra:    map[string]config.InfraEntry{"redis": {}},
		},
	}

	if err := validateStatusFilter(proj, []string{"front"}); err != nil {
		t.Errorf("front exists, should not error: %v", err)
	}

	err := validateStatusFilter(proj, []string{"front", "typo"})
	if err == nil {
		t.Fatal("expected error for unknown name")
	}
	if !strings.Contains(err.Error(), "typo") {
		t.Errorf("error must mention the typo, got: %v", err)
	}
	if !strings.Contains(err.Error(), "front") {
		t.Errorf("error must list known names for hint, got: %v", err)
	}
}

func TestValidateStatusFilter_EmptyIsNoop(t *testing.T) {
	proj := &YAMLProject{Deps: &config.Deps{}}
	if err := validateStatusFilter(proj, nil); err != nil {
		t.Errorf("nil filter must not error: %v", err)
	}
}
