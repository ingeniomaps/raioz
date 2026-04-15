package validate

import (
	"context"
	"testing"

	"raioz/internal/config"
)

func TestValidateVolumes(t *testing.T) {
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
						Docker: &config.DockerConfig{Volumes: []string{"data:/var/data"}},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "bind mount ignored for naming check",
			deps: &config.Deps{
				Services: map[string]config.Service{
					"web": {
						Docker: &config.DockerConfig{Volumes: []string{"/host:/container"}},
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
		{
			name: "invalid volume name uppercase",
			deps: &config.Deps{
				Services: map[string]config.Service{
					"web": {
						Docker: &config.DockerConfig{Volumes: []string{"BadName:/app"}},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "volume name with space",
			deps: &config.Deps{
				Services: map[string]config.Service{
					"web": {
						Docker: &config.DockerConfig{Volumes: []string{"bad name:/app"}},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "empty services",
			deps: &config.Deps{
				Services: map[string]config.Service{},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateVolumes(tt.deps, "/tmp/base", "test-project")
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateVolumes() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateNetworks(t *testing.T) {
	tests := []struct {
		name    string
		deps    *config.Deps
		wantErr bool
	}{
		{
			name: "valid network",
			deps: &config.Deps{
				Network: config.NetworkConfig{Name: "my-network"},
			},
			wantErr: false,
		},
		{
			name: "invalid network with space",
			deps: &config.Deps{
				Network: config.NetworkConfig{Name: "bad network"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateNetworks(context.Background(), tt.deps)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateNetworks() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateServicePath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{"valid relative", "services/app", false},
		{"valid dot slash", "./services/app", false},
		{"absolute rejected", "/etc/passwd", true},
		{"parent traversal rejected", "../etc", true},
		{"starts with slash rejected", "/services/app", true},
		{"empty path valid", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateServicePath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateServicePath(%q) error = %v, wantErr %v", tt.path, err, tt.wantErr)
			}
		})
	}
}

func TestValidateImageName(t *testing.T) {
	tests := []struct {
		name    string
		image   string
		wantErr bool
	}{
		{"valid simple", "nginx", false},
		{"valid with tag-ish", "postgres", false},
		{"valid registry", "docker.io/library/nginx", false},
		{"valid namespace", "bitnami/postgres", false},
		{"empty rejected", "", true},
		{"backtick rejected", "nginx`ls`", true},
		{"dollar rejected", "nginx$foo", true},
		{"semicolon rejected", "nginx;rm", true},
		{"pipe rejected", "nginx|cat", true},
		{"ampersand rejected", "nginx&bad", true},
		{"newline rejected", "nginx\nrm", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateImageName(tt.image)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateImageName(%q) error = %v, wantErr %v", tt.image, err, tt.wantErr)
			}
		})
	}

	// Too long
	long := make([]byte, 260)
	for i := range long {
		long[i] = 'a'
	}
	if err := ValidateImageName(string(long)); err == nil {
		t.Error("expected error for 260-char image name, got nil")
	}
}
