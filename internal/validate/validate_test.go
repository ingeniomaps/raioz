package validate

import (
	"testing"

	"raioz/internal/domain/models"
)

func TestValidateProject(t *testing.T) {
	tests := []struct {
		name    string
		deps    *models.Deps
		wantErr bool
	}{
		{
			name: "valid project",
			deps: &models.Deps{
				SchemaVersion: "1.0",
				Network:       models.NetworkConfig{Name: "test-network"},
				Project: models.Project{
					Name: "test-project",
				},
				Services: map[string]models.Service{
					"test": {
						Source: models.SourceConfig{
							Kind:   "git",
							Repo:   "git@github.com:test/repo.git",
							Branch: "main",
							Path:   "services/test",
						},
						Docker: &models.DockerConfig{
							Mode:       "dev",
							Dockerfile: "Dockerfile",
							Ports:      []string{},
							Volumes:    []string{},
							DependsOn:  []string{},
						},
						Env: &models.EnvValue{Files: []string{}},
					},
				},
				Infra: map[string]models.InfraEntry{},
				Env: models.EnvConfig{
					UseGlobal: true,
					Files:     []string{},
				},
			},
			wantErr: false,
		},
		{
			name: "missing project name",
			deps: &models.Deps{
				SchemaVersion: "1.0",
				Network:       models.NetworkConfig{Name: "test-network"},
				Project:       models.Project{},
				Services:      map[string]models.Service{},
				Infra:         map[string]models.InfraEntry{},
				Env:           models.EnvConfig{},
			},
			wantErr: true,
		},
		{
			name: "missing service source fields for git",
			deps: &models.Deps{
				SchemaVersion: "1.0",
				Network:       models.NetworkConfig{Name: "test"},
				Project: models.Project{
					Name: "test",
				},
				Services: map[string]models.Service{
					"test": {
						Source: models.SourceConfig{
							Kind: "git",
							// Missing repo, branch, path
						},
						Docker: &models.DockerConfig{
							Mode: "dev",
						},
					},
				},
				Infra: map[string]models.InfraEntry{},
				Env:   models.EnvConfig{},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := All(tt.deps)
			if (err != nil) != tt.wantErr {
				t.Errorf("All() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
