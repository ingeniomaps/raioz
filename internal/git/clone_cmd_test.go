package git

import (
	"context"
	"testing"

	"raioz/internal/domain/models"
)

// No branch means the remote's default one: `-b ""` makes git fail.
func TestNewAuthenticatedCloneCmd_DefaultBranch(t *testing.T) {
	tests := []struct {
		name   string
		branch string
		wantB  bool
	}{
		{"explicit branch", "feat", true},
		{"default branch", "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			src := models.SourceConfig{Repo: "https://github.com/acme/app.git", Branch: tc.branch}
			cmd, cleanup, err := newAuthenticatedCloneCmd(context.Background(), src, t.TempDir())
			if err != nil {
				t.Fatalf("newAuthenticatedCloneCmd: %v", err)
			}
			defer cleanup()
			hasB := false
			for _, arg := range cmd.Args {
				if arg == "-b" {
					hasB = true
				}
				if arg == "" {
					t.Errorf("empty argument in %v", cmd.Args)
				}
			}
			if hasB != tc.wantB {
				t.Errorf("-b present = %v, want %v (%v)", hasB, tc.wantB, cmd.Args)
			}
		})
	}
}
