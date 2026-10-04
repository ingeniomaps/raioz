package git

import (
	"strings"
	"testing"

	"raioz/internal/domain/models"
	"raioz/internal/i18n"
)

func TestExplainCloneFailure(t *testing.T) {
	i18n.Init("en")
	repo := "https://github.com/acme/api"
	tests := []struct {
		name, stderr string
		src          models.SourceConfig
		want         []string // substrings; nil = not explained
	}{
		{
			"private repo, no auth declared",
			"fatal: could not read Username for 'https://github.com': terminal prompts disabled",
			models.SourceConfig{Repo: repo},
			[]string{repo, "auth: gh", "auth: ssh", "auth: inherit"},
		},
		{
			"declared provider rejected",
			"git@github.com: Permission denied (publickey).\nfatal: Could not read from remote repository.",
			models.SourceConfig{Repo: repo, Auth: "ssh"},
			[]string{repo, "auth: ssh", "ssh-agent"},
		},
		{
			"gh without access",
			"remote: Invalid username or token.\nfatal: Authentication failed for 'https://github.com/acme/api/'",
			models.SourceConfig{Repo: repo, Auth: "gh"},
			[]string{"auth: gh", "gh auth login"},
		},
		{
			"missing branch",
			"warning: Could not find remote branch nope to clone.\nfatal: Remote branch nope not found in upstream origin",
			models.SourceConfig{Repo: repo, Branch: "nope"},
			[]string{`"nope"`, repo},
		},
		{
			"anything else is left to git's own words",
			"fatal: unable to access 'https://github.com/acme/api/': Could not resolve host: github.com",
			models.SourceConfig{Repo: repo},
			nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := explainCloneFailure(tt.stderr, tt.src)
			if tt.want == nil {
				if got != "" {
					t.Errorf("want no explanation, got %q", got)
				}
				return
			}
			for _, w := range tt.want {
				if !strings.Contains(got, w) {
					t.Errorf("explanation %q misses %q", got, w)
				}
			}
		})
	}
}
