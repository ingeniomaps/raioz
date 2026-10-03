package host

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"raioz/internal/domain/models"
	"raioz/internal/workspace"
)

// resolveEnvVars returns KEY=VALUE entries in the order the child sees
// them, where the last one wins.
func TestResolveEnvVars_Precedence(t *testing.T) {
	projectDir := t.TempDir()
	envFile := filepath.Join(projectDir, ".env")
	if err := os.WriteFile(envFile, []byte("TAG=file\nCACHE_URL=file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	ws := &workspace.Workspace{Root: projectDir, EnvDir: filepath.Join(t.TempDir(), "env")}
	deps := &models.Deps{Project: models.Project{Name: "bencha"}}
	computed := map[string]string{"CACHE_URL": "computed", "PORT": "38101"}

	last := func(env []string, key string) string {
		val := ""
		for _, kv := range env {
			if v, ok := strings.CutPrefix(kv, key+"="); ok {
				val = v
			}
		}
		return val
	}

	tests := []struct {
		name string
		ctx  context.Context
		env  *models.EnvValue
		want map[string]string
	}{
		{
			name: "computed vars beat the env file",
			ctx:  WithExtraEnv(context.Background(), computed),
			env:  &models.EnvValue{Files: []string{envFile}},
			want: map[string]string{"TAG": "file", "CACHE_URL": "computed", "PORT": "38101"},
		},
		{
			name: "no extra env keeps the file values",
			ctx:  context.Background(),
			env:  &models.EnvValue{Files: []string{envFile}},
			want: map[string]string{"TAG": "file", "CACHE_URL": "file", "PORT": ""},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := models.Service{Env: tc.env}
			got, err := resolveEnvVars(tc.ctx, ws, deps, "web", svc, projectDir, projectDir)
			if err != nil {
				t.Fatalf("resolveEnvVars: %v", err)
			}
			for k, want := range tc.want {
				if v := last(got, k); v != want {
					t.Errorf("%s = %q, want %q", k, v, want)
				}
			}
		})
	}
}
