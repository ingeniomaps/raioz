package app

import (
	"context"
	"errors"
	"testing"

	"raioz/internal/domain/models"
)

// benchx asks for 38104 (a host service) and 36379 (a published
// dependency), in workspace "acme" where it also declares `postgres`.
func conflictDeps() *models.Deps {
	return &models.Deps{
		Project:   models.Project{Name: "benchx"},
		Workspace: "acme",
		Services: map[string]models.Service{
			"py": {Source: models.SourceConfig{Kind: "local", Path: "py", Command: "python3 server.py"}, Port: 38104},
		},
		Infra: map[string]models.InfraEntry{
			"kvx":      {Inline: &models.Infra{Image: "redis", Tag: "7-alpine", Ports: []string{"36379:6379"}}},
			"postgres": {Inline: &models.Infra{Image: "postgres", Tag: "16", Ports: []string{"5432:5432"}}},
		},
	}
}

func TestFindPortConflicts(t *testing.T) {
	tests := []struct {
		name   string
		active []activeEndpoint
		want   map[string]string // port → owner
	}{
		{
			name: "another project's host service and container both conflict",
			active: []activeEndpoint{
				{Project: "bencha", Service: "py", Runner: runnerHost, Port: 38104},
				{Project: "bencha", Service: "cache", Runner: runnerContainer, Port: 36379},
			},
			want: map[string]string{"38104": "bencha", "36379": "bencha"},
		},
		{
			name: "the cwd project's own endpoints are not conflicts",
			active: []activeEndpoint{
				{Project: "benchx", Service: "py", Runner: runnerHost, Port: 38104},
				{Project: "benchx", Service: "kvx", Runner: runnerContainer, Port: 36379},
			},
			want: map[string]string{},
		},
		{
			name: "a shared dependency of the same workspace is reused, not a conflict",
			active: []activeEndpoint{
				{Workspace: "acme", Service: "postgres", Runner: runnerContainer, Port: 5432},
			},
			want: map[string]string{},
		},
		{
			name: "another workspace's shared dependency is one, named by workspace",
			active: []activeEndpoint{
				{Workspace: "globex", Service: "postgres", Runner: runnerContainer, Port: 5432},
			},
			want: map[string]string{"5432": "globex"},
		},
		{
			name: "a port nobody asked for is ignored",
			active: []activeEndpoint{
				{Project: "bencha", Service: "web", Runner: runnerHost, Port: 38101},
			},
			want: map[string]string{},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			stubActiveEndpoints(t, tc.active)
			conflicts, err := findPortConflicts(context.Background(), conflictDeps())
			if err != nil {
				t.Fatalf("findPortConflicts: %v", err)
			}
			got := map[string]string{}
			for _, c := range conflicts {
				got[c.Port] = c.Project
			}
			if len(got) != len(tc.want) {
				t.Fatalf("conflicts = %v, want %v", got, tc.want)
			}
			for port, owner := range tc.want {
				if got[port] != owner {
					t.Errorf("port %s held by %q, want %q", port, got[port], owner)
				}
			}
		})
	}
}

// Another project is stopped through its own `raioz down`: removing its
// containers would leave its host services running on the very ports the
// caller wants.
func TestStopProjects_UsesTheProjectsOwnDown(t *testing.T) {
	initI18nForTest(t)
	stubs := &downOthersStubs{stopped: map[string][]string{"legacy": {"legacy-api"}}}
	withDownOthersHooks(t, stubs)

	projectPathFn = func(name string) string {
		return map[string]string{"bencha": "/work/bencha", "broken": "/work/broken"}[name]
	}
	var downed []string
	downProjectFn = func(_ context.Context, dir string) error {
		if dir == "/work/broken" {
			return errors.New("down failed")
		}
		downed = append(downed, dir)
		return nil
	}

	stopped := stopProjects(context.Background(), []string{"bencha", "legacy", "broken"})

	if len(downed) != 1 || downed[0] != "/work/bencha" {
		t.Errorf("down ran in %v, want only /work/bencha", downed)
	}
	if len(stopped) != 2 || stopped[0] != "bencha" || stopped[1] != "legacy" {
		t.Errorf("stopped = %v, want [bencha legacy]", stopped)
	}
	// bencha went down by its own down; the other two fell back to the
	// container sweep (legacy has no path, broken's down failed).
	if len(stubs.stopProjectCalls) != 2 {
		t.Errorf("container fallback called for %v", stubs.stopProjectCalls)
	}
}

// A project made only of host services runs no container, so Docker does
// not list it; the state does.
func TestDownAllOtherProjects_IncludesHostOnlyProjects(t *testing.T) {
	initI18nForTest(t)
	stubs := &downOthersStubs{active: []string{"withcontainers"}}
	withDownOthersHooks(t, stubs)
	recordedProjects = func() []models.ProjectState {
		return []models.ProjectState{{Name: "hostonly", Path: "/work/hostonly"}, {Name: "me", Path: "/work/me"}}
	}
	projectPathFn = func(name string) string {
		return map[string]string{"hostonly": "/work/hostonly", "me": "/work/me"}[name]
	}
	var downed []string
	downProjectFn = func(_ context.Context, dir string) error {
		downed = append(downed, dir)
		return nil
	}

	stopped, err := DownAllOtherProjects(context.Background(), "me", approveAll)
	if err != nil {
		t.Fatalf("DownAllOtherProjects: %v", err)
	}
	if len(downed) != 1 || downed[0] != "/work/hostonly" {
		t.Errorf("down ran in %v, want only the host-only project (never the cwd one)", downed)
	}
	if len(stopped) != 1 || stopped[0] != "hostonly" {
		t.Errorf("stopped = %v, want [hostonly]", stopped)
	}
}
