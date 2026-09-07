package state

import (
	"testing"

	"raioz/internal/domain/models"
)

func TestCompareDeps(t *testing.T) {
	tests := []struct {
		name        string
		oldDeps     *models.Deps
		newDeps     *models.Deps
		wantChanges int
	}{
		{
			name:        "no previous state",
			oldDeps:     nil,
			newDeps:     &models.Deps{},
			wantChanges: 0,
		},
		{
			name: "same config",
			oldDeps: &models.Deps{
				Network: models.NetworkConfig{Name: "test-net"},
				Project: models.Project{Name: "test"},
			},
			newDeps: &models.Deps{
				Network: models.NetworkConfig{Name: "test-net"},
				Project: models.Project{Name: "test"},
			},
			wantChanges: 0,
		},
		{
			name: "network changed",
			oldDeps: &models.Deps{
				Network: models.NetworkConfig{Name: "old-net"},
				Project: models.Project{Name: "test"},
			},
			newDeps: &models.Deps{
				Network: models.NetworkConfig{Name: "new-net"},
				Project: models.Project{Name: "test"},
			},
			wantChanges: 1,
		},
		{
			name: "service branch changed",
			oldDeps: &models.Deps{
				Services: map[string]models.Service{
					"service1": {
						Source: models.SourceConfig{Kind: "git", Branch: "main"},
					},
				},
			},
			newDeps: &models.Deps{
				Services: map[string]models.Service{
					"service1": {
						Source: models.SourceConfig{Kind: "git", Branch: "develop"},
					},
				},
			},
			wantChanges: 1,
		},
		{
			name: "service added",
			oldDeps: &models.Deps{
				Services: map[string]models.Service{},
			},
			newDeps: &models.Deps{
				Services: map[string]models.Service{
					"service1": {
						Source: models.SourceConfig{Kind: "git"},
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
