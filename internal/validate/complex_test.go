package validate

import (
	"testing"

	"raioz/internal/config"
)

func TestValidateProfileConsistency(t *testing.T) {
	tests := []struct {
		name    string
		deps    *config.Deps
		wantErr bool
	}{
		{
			name: "valid profiles",
			deps: &config.Deps{
				Profiles: []string{"frontend", "backend", "load-balancer"},
				Services: map[string]config.Service{
					"web": {Profiles: []string{"frontend"}},
				},
				Infra: map[string]config.InfraEntry{
					"db": {Inline: &config.Infra{Image: "postgres", Profiles: []string{"backend"}}},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid root profile uppercase",
			deps: &config.Deps{
				Profiles: []string{"Frontend"},
				Services: map[string]config.Service{},
				Infra:    map[string]config.InfraEntry{},
			},
			wantErr: true,
		},
		{
			name: "invalid service profile with underscore",
			deps: &config.Deps{
				Services: map[string]config.Service{
					"web": {Profiles: []string{"bad_profile"}},
				},
				Infra: map[string]config.InfraEntry{},
			},
			wantErr: true,
		},
		{
			name: "invalid infra profile with space",
			deps: &config.Deps{
				Services: map[string]config.Service{},
				Infra: map[string]config.InfraEntry{
					"db": {Inline: &config.Infra{Image: "postgres", Profiles: []string{"bad profile"}}},
				},
			},
			wantErr: true,
		},
		{
			name: "empty profiles everywhere",
			deps: &config.Deps{
				Services: map[string]config.Service{"web": {}},
				Infra:    map[string]config.InfraEntry{},
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
		deps    *config.Deps
		wantErr bool
	}{
		{
			name: "valid relative env files",
			deps: &config.Deps{
				Env: config.EnvConfig{
					Files: []string{"global.env", "services/web.env"},
				},
			},
			wantErr: false,
		},
		{
			name: "empty env files list",
			deps: &config.Deps{
				Env: config.EnvConfig{Files: []string{}},
			},
			wantErr: false,
		},
		{
			name: "empty string env file is skipped",
			deps: &config.Deps{
				Env: config.EnvConfig{Files: []string{""}},
			},
			wantErr: false,
		},
		{
			name: "absolute env file path rejected",
			deps: &config.Deps{
				Env: config.EnvConfig{Files: []string{"/etc/secrets.env"}},
			},
			wantErr: true,
		},
		{
			name: "env file with parent traversal rejected",
			deps: &config.Deps{
				Env: config.EnvConfig{Files: []string{"../escape.env"}},
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
		deps    *config.Deps
		wantErr bool
	}{
		{
			name: "valid named volume",
			deps: &config.Deps{
				Services: map[string]config.Service{
					"web": {
						Docker: &config.DockerConfig{
							Volumes: []string{"data:/var/data"},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid bind mount",
			deps: &config.Deps{
				Services: map[string]config.Service{
					"web": {
						Docker: &config.DockerConfig{
							Volumes: []string{"/host/path:/container/path:ro"},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "relative destination rejected",
			deps: &config.Deps{
				Services: map[string]config.Service{
					"web": {
						Docker: &config.DockerConfig{
							Volumes: []string{"data:app/data"},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "bind source with parent traversal rejected",
			deps: &config.Deps{
				Services: map[string]config.Service{
					"web": {
						Docker: &config.DockerConfig{
							Volumes: []string{"/host/../secret:/container/path"},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "skip invalid format (less than 2 parts)",
			deps: &config.Deps{
				Services: map[string]config.Service{
					"web": {
						Docker: &config.DockerConfig{
							Volumes: []string{"justaname"},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "nil docker skipped",
			deps: &config.Deps{
				Services: map[string]config.Service{
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
		deps    *config.Deps
		wantErr bool
	}{
		{
			name: "valid minimal config",
			deps: &config.Deps{
				SchemaVersion: "1.0",
				Network:       config.NetworkConfig{Name: "test-net"},
				Project:       config.Project{Name: "test"},
				Services: map[string]config.Service{
					"web": {
						Source: config.SourceConfig{Kind: "image", Image: "nginx", Tag: "latest"},
						Docker: &config.DockerConfig{Mode: "dev"},
					},
				},
				Infra: map[string]config.InfraEntry{},
				Env:   config.EnvConfig{},
			},
			wantErr: false,
		},
		{
			name: "invalid profile fails complex",
			deps: &config.Deps{
				SchemaVersion: "1.0",
				Network:       config.NetworkConfig{Name: "test-net"},
				Project:       config.Project{Name: "test"},
				Profiles:      []string{"BadProfile"},
				Services: map[string]config.Service{
					"web": {
						Source: config.SourceConfig{Kind: "image", Image: "nginx", Tag: "latest"},
						Docker: &config.DockerConfig{Mode: "dev"},
					},
				},
				Infra: map[string]config.InfraEntry{},
			},
			wantErr: true,
		},
		{
			name: "invalid env reference fails complex",
			deps: &config.Deps{
				SchemaVersion: "1.0",
				Network:       config.NetworkConfig{Name: "test-net"},
				Project:       config.Project{Name: "test"},
				Services: map[string]config.Service{
					"web": {
						Source: config.SourceConfig{Kind: "image", Image: "nginx", Tag: "latest"},
						Docker: &config.DockerConfig{Mode: "dev"},
					},
				},
				Infra: map[string]config.InfraEntry{},
				Env:   config.EnvConfig{Files: []string{"/absolute/path.env"}},
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
