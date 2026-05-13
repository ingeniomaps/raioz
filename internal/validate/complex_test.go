package validate

import (
	"testing"

	"raioz/internal/domain/models"
)

func TestValidateProfileConsistency(t *testing.T) {
	tests := []struct {
		name    string
		deps    *models.Deps
		wantErr bool
	}{
		{
			name: "valid profiles",
			deps: &models.Deps{
				Profiles: []string{"frontend", "backend", "load-balancer"},
				Services: map[string]models.Service{
					"web": {Profiles: []string{"frontend"}},
				},
				Infra: map[string]models.InfraEntry{
					"db": {Inline: &models.Infra{Image: "postgres", Profiles: []string{"backend"}}},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid root profile uppercase",
			deps: &models.Deps{
				Profiles: []string{"Frontend"},
				Services: map[string]models.Service{},
				Infra:    map[string]models.InfraEntry{},
			},
			wantErr: true,
		},
		{
			name: "invalid service profile with underscore",
			deps: &models.Deps{
				Services: map[string]models.Service{
					"web": {Profiles: []string{"bad_profile"}},
				},
				Infra: map[string]models.InfraEntry{},
			},
			wantErr: true,
		},
		{
			name: "invalid infra profile with space",
			deps: &models.Deps{
				Services: map[string]models.Service{},
				Infra: map[string]models.InfraEntry{
					"db": {Inline: &models.Infra{Image: "postgres", Profiles: []string{"bad profile"}}},
				},
			},
			wantErr: true,
		},
		{
			name: "empty profiles everywhere",
			deps: &models.Deps{
				Services: map[string]models.Service{"web": {}},
				Infra:    map[string]models.InfraEntry{},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateProfileConsistency(tt.deps)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateProfileConsistency() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateEnvReferences(t *testing.T) {
	tests := []struct {
		name    string
		deps    *models.Deps
		wantErr bool
	}{
		{
			name: "valid relative env files",
			deps: &models.Deps{
				Env: models.EnvConfig{
					Files: []string{"global.env", "services/web.env"},
				},
			},
			wantErr: false,
		},
		{
			name: "empty env files list",
			deps: &models.Deps{
				Env: models.EnvConfig{Files: []string{}},
			},
			wantErr: false,
		},
		{
			name: "empty string env file is skipped",
			deps: &models.Deps{
				Env: models.EnvConfig{Files: []string{""}},
			},
			wantErr: false,
		},
		{
			name: "absolute env file path rejected",
			deps: &models.Deps{
				Env: models.EnvConfig{Files: []string{"/etc/secrets.env"}},
			},
			wantErr: true,
		},
		{
			name: "env file with parent traversal rejected",
			deps: &models.Deps{
				Env: models.EnvConfig{Files: []string{"../escape.env"}},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateEnvReferences(tt.deps)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateEnvReferences() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateVolumeMounts(t *testing.T) {
	tests := []struct {
		name    string
		deps    *models.Deps
		wantErr bool
	}{
		{
			name: "valid named volume",
			deps: &models.Deps{
				Services: map[string]models.Service{
					"web": {
						Docker: &models.DockerConfig{
							Volumes: []string{"data:/var/data"},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid bind mount",
			deps: &models.Deps{
				Services: map[string]models.Service{
					"web": {
						Docker: &models.DockerConfig{
							Volumes: []string{"/host/path:/container/path:ro"},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "relative destination rejected",
			deps: &models.Deps{
				Services: map[string]models.Service{
					"web": {
						Docker: &models.DockerConfig{
							Volumes: []string{"data:app/data"},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "bind source with parent traversal rejected",
			deps: &models.Deps{
				Services: map[string]models.Service{
					"web": {
						Docker: &models.DockerConfig{
							Volumes: []string{"/host/../secret:/container/path"},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "skip invalid format (less than 2 parts)",
			deps: &models.Deps{
				Services: map[string]models.Service{
					"web": {
						Docker: &models.DockerConfig{
							Volumes: []string{"justaname"},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "nil docker skipped",
			deps: &models.Deps{
				Services: map[string]models.Service{
					"web": {Docker: nil},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateVolumeMounts(tt.deps)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateVolumeMounts() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateComplexConfiguration(t *testing.T) {
	tests := []struct {
		name    string
		deps    *models.Deps
		wantErr bool
	}{
		{
			name: "valid minimal config",
			deps: &models.Deps{
				SchemaVersion: "1.0",
				Network:       models.NetworkConfig{Name: "test-net"},
				Project:       models.Project{Name: "test"},
				Services: map[string]models.Service{
					"web": {
						Source: models.SourceConfig{Kind: "image", Image: "nginx", Tag: "latest"},
						Docker: &models.DockerConfig{Mode: "dev"},
					},
				},
				Infra: map[string]models.InfraEntry{},
				Env:   models.EnvConfig{},
			},
			wantErr: false,
		},
		{
			name: "invalid profile fails complex",
			deps: &models.Deps{
				SchemaVersion: "1.0",
				Network:       models.NetworkConfig{Name: "test-net"},
				Project:       models.Project{Name: "test"},
				Profiles:      []string{"BadProfile"},
				Services: map[string]models.Service{
					"web": {
						Source: models.SourceConfig{Kind: "image", Image: "nginx", Tag: "latest"},
						Docker: &models.DockerConfig{Mode: "dev"},
					},
				},
				Infra: map[string]models.InfraEntry{},
			},
			wantErr: true,
		},
		{
			name: "invalid env reference fails complex",
			deps: &models.Deps{
				SchemaVersion: "1.0",
				Network:       models.NetworkConfig{Name: "test-net"},
				Project:       models.Project{Name: "test"},
				Services: map[string]models.Service{
					"web": {
						Source: models.SourceConfig{Kind: "image", Image: "nginx", Tag: "latest"},
						Docker: &models.DockerConfig{Mode: "dev"},
					},
				},
				Infra: map[string]models.InfraEntry{},
				Env:   models.EnvConfig{Files: []string{"/absolute/path.env"}},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateComplexConfiguration(tt.deps)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateComplexConfiguration() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
