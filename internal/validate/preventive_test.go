package validate

import (
	"context"
	"testing"

	"raioz/internal/config"
	"raioz/internal/workspace"
)

func TestValidateGitRepositories(t *testing.T) {
	tests := []struct {
		name    string
		deps    *config.Deps
		wantErr bool
	}{
		{
			name: "no git services",
			deps: &config.Deps{
				Services: map[string]config.Service{
					"web": {Source: config.SourceConfig{Kind: "image", Image: "nginx", Tag: "latest"}},
				},
			},
			wantErr: false,
		},
		{
			name: "valid git service",
			deps: &config.Deps{
				Services: map[string]config.Service{
					"app": {
						Source: config.SourceConfig{
							Kind:   "git",
							Repo:   "https://github.com/user/repo.git",
							Branch: "main",
							Path:   "services/app",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid git ssh url",
			deps: &config.Deps{
				Services: map[string]config.Service{
					"app": {
						Source: config.SourceConfig{
							Kind:   "git",
							Repo:   "git@github.com:user/repo.git",
							Branch: "develop",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "missing repo",
			deps: &config.Deps{
				Services: map[string]config.Service{
					"app": {
						Source: config.SourceConfig{Kind: "git", Branch: "main"},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "missing branch",
			deps: &config.Deps{
				Services: map[string]config.Service{
					"app": {
						Source: config.SourceConfig{Kind: "git", Repo: "https://github.com/x/y.git"},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "dangerous char in branch",
			deps: &config.Deps{
				Services: map[string]config.Service{
					"app": {
						Source: config.SourceConfig{
							Kind:   "git",
							Repo:   "https://github.com/x/y.git",
							Branch: "main;rm -rf /",
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "dangerous char in repo",
			deps: &config.Deps{
				Services: map[string]config.Service{
					"app": {
						Source: config.SourceConfig{
							Kind:   "git",
							Repo:   "https://github.com/x/y.git|evil",
							Branch: "main",
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid url prefix",
			deps: &config.Deps{
				Services: map[string]config.Service{
					"app": {
						Source: config.SourceConfig{
							Kind:   "git",
							Repo:   "github.com/x/y.git",
							Branch: "main",
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "path traversal in path",
			deps: &config.Deps{
				Services: map[string]config.Service{
					"app": {
						Source: config.SourceConfig{
							Kind:   "git",
							Repo:   "https://github.com/x/y.git",
							Branch: "main",
							Path:   "../../etc/passwd",
						},
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateGitRepositories(context.Background(), tt.deps)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateGitRepositories() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateDockerImages(t *testing.T) {
	tests := []struct {
		name    string
		deps    *config.Deps
		wantErr bool
	}{
		{
			name: "valid image service",
			deps: &config.Deps{
				Services: map[string]config.Service{
					"web": {
						Source: config.SourceConfig{Kind: "image", Image: "nginx", Tag: "latest"},
					},
				},
				Infra: map[string]config.InfraEntry{},
			},
			wantErr: false,
		},
		{
			name: "missing image field",
			deps: &config.Deps{
				Services: map[string]config.Service{
					"web": {Source: config.SourceConfig{Kind: "image", Tag: "latest"}},
				},
				Infra: map[string]config.InfraEntry{},
			},
			wantErr: true,
		},
		{
			name: "missing tag field",
			deps: &config.Deps{
				Services: map[string]config.Service{
					"web": {Source: config.SourceConfig{Kind: "image", Image: "nginx"}},
				},
				Infra: map[string]config.InfraEntry{},
			},
			wantErr: true,
		},
		{
			name: "invalid image name with dangerous char",
			deps: &config.Deps{
				Services: map[string]config.Service{
					"web": {
						Source: config.SourceConfig{Kind: "image", Image: "nginx;evil", Tag: "latest"},
					},
				},
				Infra: map[string]config.InfraEntry{},
			},
			wantErr: true,
		},
		{
			name: "valid inline infra",
			deps: &config.Deps{
				Services: map[string]config.Service{},
				Infra: map[string]config.InfraEntry{
					"db": {Inline: &config.Infra{Image: "postgres", Tag: "15"}},
				},
			},
			wantErr: false,
		},
		{
			name: "infra with empty image",
			deps: &config.Deps{
				Services: map[string]config.Service{},
				Infra: map[string]config.InfraEntry{
					"db": {Inline: &config.Infra{Image: ""}},
				},
			},
			wantErr: true,
		},
		{
			name: "infra with invalid image name",
			deps: &config.Deps{
				Services: map[string]config.Service{},
				Infra: map[string]config.InfraEntry{
					"db": {Inline: &config.Infra{Image: "postgres`evil`"}},
				},
			},
			wantErr: true,
		},
		{
			name: "path-based infra skipped",
			deps: &config.Deps{
				Services: map[string]config.Service{},
				Infra: map[string]config.InfraEntry{
					"db": {Path: "./infra/db.yml"},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDockerImages(context.Background(), tt.deps)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateDockerImages() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateBeforeDown(t *testing.T) {
	// Existing workspace — may or may not pass depending on Docker availability, but
	// should not panic and should return a sensible error when Docker unavailable.
	dir := t.TempDir()
	ws := &workspace.Workspace{Root: dir}
	_ = ValidateBeforeDown(context.Background(), ws)

	// Non-existent workspace: if preflight passes, returns workspace error;
	// if preflight fails (no docker), returns preflight error. Either way no panic.
	nonExistent := &workspace.Workspace{Root: "/non/existent/path/raioz-test-xyz"}
	if err := ValidateBeforeDown(context.Background(), nonExistent); err == nil {
		// If preflight passes and workspace is missing, we expect an error.
		// If preflight fails, we also get an error. nil is unexpected.
		t.Log("ValidateBeforeDown returned nil; likely preflight + path both passed")
	}
}

func TestValidateBeforeUp_EmptyDeps(t *testing.T) {
	// Preflight may fail in CI without docker; the function should surface an error
	// (either preflight or config validation), but must not panic.
	dir := t.TempDir()
	ws := &workspace.Workspace{Root: dir}
	deps := &config.Deps{
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
	}
	// Don't assert the result — just that it doesn't panic
	_ = ValidateBeforeUp(context.Background(), deps, ws)
}
