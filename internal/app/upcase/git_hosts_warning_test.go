package upcase

import (
	"context"
	"strings"
	"testing"

	"raioz/internal/domain/models"
	"raioz/internal/i18n"
)

// No test of this package dials a git host: the probe is off unless a
// test turns it on.
func init() {
	unreachableGitHostsFn = func(context.Context, []string) []string { return nil }
}

func TestWarnUnreachableGitHosts(t *testing.T) {
	i18n.Init("en")
	git := func(repo string) models.Service {
		return models.Service{Source: models.SourceConfig{Kind: "git", Repo: repo}}
	}
	local := models.Service{Source: models.SourceConfig{Kind: "local", Path: "./web"}}

	var asked [][]string
	prev := unreachableGitHostsFn
	unreachableGitHostsFn = func(_ context.Context, repos []string) []string {
		asked = append(asked, repos)
		return []string{"git.acme.dev"}
	}
	t.Cleanup(func() { unreachableGitHostsFn = prev })

	t.Run("a project that clones nothing is not asked", func(t *testing.T) {
		asked = nil
		out := captureStdoutForLog(t, func() {
			warnUnreachableGitHosts(context.Background(), &models.Deps{
				Services: map[string]models.Service{"web": local},
			})
		})
		if len(asked) != 0 || out != "" {
			t.Errorf("asked %v, printed %q", asked, out)
		}
	})

	t.Run("an unreachable git host is a warning, not a failure", func(t *testing.T) {
		asked = nil
		out := captureStdoutForLog(t, func() {
			warnUnreachableGitHosts(context.Background(), &models.Deps{
				Services: map[string]models.Service{"web": local, "api": git("git@git.acme.dev:acme/api.git")},
			})
		})
		if len(asked) != 1 || len(asked[0]) != 1 {
			t.Errorf("only the git service's remote is asked, got %v", asked)
		}
		if !strings.Contains(out, "git.acme.dev") {
			t.Errorf("warning must name the host, got %q", out)
		}
	})
}
