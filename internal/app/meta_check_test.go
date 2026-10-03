package app

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"raioz/internal/config"
)

func writeSubProject(t *testing.T, dir, body string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "raioz.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCheckMeta(t *testing.T) {
	const valid = "project: sub\ndependencies:\n  kv:\n    image: redis:7\n"

	tests := []struct {
		name    string
		project func(root string) config.MetaProject
		wantErr bool
	}{
		{
			name: "sub-project with a loadable config",
			project: func(root string) config.MetaProject {
				writeSubProject(t, filepath.Join(root, "ok"), valid)
				return config.MetaProject{Name: "ok", Path: filepath.Join(root, "ok"), Mode: config.MetaModeLocal}
			},
		},
		{
			name: "sub-project whose config does not load",
			project: func(root string) config.MetaProject {
				writeSubProject(t, filepath.Join(root, "bad"), "project: bad\n")
				return config.MetaProject{Name: "bad", Path: filepath.Join(root, "bad"), Mode: config.MetaModeLocal}
			},
			wantErr: true,
		},
		{
			name: "required sub-project missing from disk",
			project: func(root string) config.MetaProject {
				return config.MetaProject{Name: "gone", Path: filepath.Join(root, "gone"), Mode: config.MetaModeLocal}
			},
			wantErr: true,
		},
		{
			name: "optional sub-project missing from disk",
			project: func(root string) config.MetaProject {
				return config.MetaProject{Name: "opt", Path: filepath.Join(root, "opt"), Mode: config.MetaModeSkip}
			},
		},
		{
			name: "sub-project up would clone",
			project: func(root string) config.MetaProject {
				return config.MetaProject{
					Name: "cl", Path: filepath.Join(root, "cl"), Mode: config.MetaModeClone, Git: "git@example.com:a/b.git",
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.MetaConfig{Projects: []config.MetaProject{tt.project(t.TempDir())}}
			err := CheckMeta(cfg)
			if (err != nil) != tt.wantErr {
				t.Fatalf("CheckMeta error = %v, wantErr %v", err, tt.wantErr)
			}
			var exit *ExitCodeError
			if tt.wantErr && (!errors.As(err, &exit) || exit.Code != 1) {
				t.Errorf("want an exit-code-1 error, got %v", err)
			}
		})
	}
}
