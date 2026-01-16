package state

import (
	"raioz/internal/config"
	"testing"
)

func TestCompareDeps(t *testing.T) {
	tests := []struct {
		name     string
		oldDeps  *config.Deps
		newDeps  *config.Deps
		wantChanges int
	}{
		{
			name:     "no previous state",
			oldDeps:  nil,
			newDeps:  &config.Deps{},
			wantChanges: 0,
		},
		{
			name: "same config",
			oldDeps: &config.Deps{
				Project: config.Project{
					Name:    "test",
					Network: "test-net",
				},
			},
			newDeps: &config.Deps{
				Project: config.Project{
					Name:    "test",
					Network: "test-net",
				},
			},
			wantChanges: 0,
		},
		{
			name: "network changed",
			oldDeps: &config.Deps{
				Project: config.Project{
					Name:    "test",
					Network: "old-net",
				},
			},
			newDeps: &config.Deps{
				Project: config.Project{
					Name:    "test",
					Network: "new-net",
				},
			},
			wantChanges: 1,
		},
		{
			name: "service branch changed",
			oldDeps: &config.Deps{
				Services: map[string]config.Service{
					"service1": {
						Source: config.SourceConfig{
							Kind:   "git",
							Branch: "main",
						},
					},
				},
			},
			newDeps: &config.Deps{
				Services: map[string]config.Service{
					"service1": {
						Source: config.SourceConfig{
							Kind:   "git",
							Branch: "develop",
						},
					},
				},
			},
			wantChanges: 1,
		},
		{
			name: "service added",
			oldDeps: &config.Deps{
				Services: map[string]config.Service{},
			},
			newDeps: &config.Deps{
				Services: map[string]config.Service{
					"service1": {
						Source: config.SourceConfig{
							Kind: "git",
						},
					},
				},
			},
			wantChanges: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			changes, err := CompareDeps(tt.oldDeps, tt.newDeps)
			if err != nil {
				t.Fatalf("CompareDeps() error = %v", err)
			}

			if len(changes) != tt.wantChanges {
				t.Errorf("CompareDeps() changes = %d, want %d", len(changes), tt.wantChanges)
			}
		})
	}
}

func TestHasSignificantChanges(t *testing.T) {
	tests := []struct {
		name     string
		changes  []ConfigChange
		wantSignificant bool
	}{
		{
			name:     "no changes",
			changes:  []ConfigChange{},
			wantSignificant: false,
		},
		{
			name: "branch change",
			changes: []ConfigChange{
				{
					Field: "source.branch",
					OldValue: "main",
					NewValue: "develop",
				},
			},
			wantSignificant: true,
		},
		{
			name: "ports change",
			changes: []ConfigChange{
				{
					Field: "docker.ports",
					OldValue: "[3000:3000]",
					NewValue: "[3001:3000]",
				},
			},
			wantSignificant: true,
		},
		{
			name: "minor change",
			changes: []ConfigChange{
				{
					Field: "env",
					OldValue: "[]",
					NewValue: "[\"test\"]",
				},
			},
			wantSignificant: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HasSignificantChanges(tt.changes)
			if got != tt.wantSignificant {
				t.Errorf("HasSignificantChanges() = %v, want %v", got, tt.wantSignificant)
			}
		})
	}
}

func TestFormatChanges(t *testing.T) {
	changes := []ConfigChange{
		{
			Type:     "service",
			Name:     "service1",
			Field:    "source.branch",
			OldValue: "main",
			NewValue: "develop",
		},
		{
			Type:     "service",
			Name:     "service2",
			Field:    "added",
			OldValue: "",
			NewValue: "new service",
		},
	}

	formatted := FormatChanges(changes)
	if formatted == "" {
		t.Error("FormatChanges() should return formatted string")
	}
	if len(formatted) < 10 {
		t.Error("FormatChanges() should return meaningful output")
	}
}
