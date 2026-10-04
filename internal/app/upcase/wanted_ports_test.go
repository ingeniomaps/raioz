package upcase

import (
	"testing"

	"raioz/internal/domain/models"
)

// The collisions `ports --conflicting` and `switch` look for are the ones
// the allocator hides by bumping: every wanted port is reported as written
// even when something already holds it.
func TestWantedHostPorts_IgnoresWhatIsBound(t *testing.T) {
	prev := portInUseProbe
	portInUseProbe = func(string) (bool, error) { return true, nil }
	t.Cleanup(func() { portInUseProbe = prev })

	deps := &models.Deps{
		Project: models.Project{Name: "benchx"},
		Services: map[string]models.Service{
			"py":  {Source: models.SourceConfig{Kind: "local", Path: "py", Command: "python3 server.py"}, Port: 38104},
			"api": {Source: models.SourceConfig{Kind: "local", Path: t.TempDir()}, Port: 3000, Docker: nil},
		},
		Infra: map[string]models.InfraEntry{
			"kvx":   {Inline: &models.Infra{Image: "redis", Tag: "7-alpine", Ports: []string{"36379:6379"}}},
			"quiet": {Inline: &models.Infra{Image: "redis", Tag: "7-alpine"}},
		},
	}

	got := map[int]string{}
	for _, w := range WantedHostPorts(deps) {
		got[w.Port] = w.Name
	}
	want := map[int]string{38104: "py", 36379: "kvx"}
	for port, name := range want {
		if got[port] != name {
			t.Errorf("port %d → %q, want %q (all: %v)", port, got[port], name, got)
		}
	}
	for port, name := range got {
		if name == "quiet" {
			t.Errorf("unpublished dependency reported on port %d", port)
		}
	}
}
