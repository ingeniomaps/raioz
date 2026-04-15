package validate

import (
	"testing"

	"raioz/internal/config"
)

func TestValidateProject(t *testing.T) {
	tests := []struct {
		name    string
		deps    *config.Deps
		wantErr bool
	}{
		{
			name: "valid project",
			deps: &config.Deps{
				SchemaVersion: "1.0",
				Network:       config.NetworkConfig{Name: "test-network"},
				Project: config.Project{
					Name: "test-project",
				},
				Services: map[string]config.Service{
					"test": {
						Source: config.SourceConfig{
							Kind:   "git",
							Repo:   "git@github.com:test/repo.git",
							Branch: "main",
							Path:   "services/test",
						},
						Docker: &config.DockerConfig{
							Mode:       "dev",
							Dockerfile: "Dockerfile",
							Ports:      []string{},
							Volumes:    []string{},
							DependsOn:  []string{},
						},
						Env: &config.EnvValue{Files: []string{}},
					},
				},
				Infra: map[string]config.InfraEntry{},
				Env: config.EnvConfig{
					UseGlobal: true,
					Files:     []string{},
				},
			},
			wantErr: false,
		},
		{
			name: "missing project name",
			deps: &config.Deps{
				SchemaVersion: "1.0",
				Network:       config.NetworkConfig{Name: "test-network"},
				Project:       config.Project{},
				Services:      map[string]config.Service{},
				Infra:         map[string]config.InfraEntry{},
				Env:           config.EnvConfig{},
			},
			wantErr: true,
		},
		{
			name: "missing service source fields for git",
			deps: &config.Deps{
				SchemaVersion: "1.0",
				Network:       config.NetworkConfig{Name: "test"},
				Project: config.Project{
					Name: "test",
				},
				Services: map[string]config.Service{
					"test": {
						Source: config.SourceConfig{
							Kind: "git",
							// Missing repo, branch, path
						},
						Docker: &config.DockerConfig{
							Mode: "dev",
						},
					},
				},
				Infra: map[string]config.InfraEntry{},
				Env:   config.EnvConfig{},
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
