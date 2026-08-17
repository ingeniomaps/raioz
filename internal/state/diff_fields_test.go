package state

import (
	"strings"
	"testing"

	"raioz/internal/domain/models"
)

func svcWithDocker(d *models.DockerConfig) models.Service {
	return models.Service{Docker: d}
}

// The drift report is what tells a user why `up` is about to recreate a
// container. A field that changes without being reported means a silent
// stale container; one reported wrongly means a needless recreation.
func TestCompareServiceFields(t *testing.T) {
	cases := []struct {
		name      string
		old, new  models.Service
		wantField string
		wantOld   string
		wantNew   string
	}{
		{
			name:      "git branch",
			old:       models.Service{Source: models.SourceConfig{Branch: "main"}},
			new:       models.Service{Source: models.SourceConfig{Branch: "next"}},
			wantField: "source.branch", wantOld: "main", wantNew: "next",
		},
		{
			name:      "image tag",
			old:       models.Service{Source: models.SourceConfig{Tag: "1.0"}},
			new:       models.Service{Source: models.SourceConfig{Tag: "1.1"}},
			wantField: "source.tag", wantOld: "1.0", wantNew: "1.1",
		},
		{
			name:      "image itself",
			old:       models.Service{Source: models.SourceConfig{Image: "nginx"}},
			new:       models.Service{Source: models.SourceConfig{Image: "caddy"}},
			wantField: "source.image", wantOld: "nginx", wantNew: "caddy",
		},
		{
			name:      "service-level dependsOn",
			old:       models.Service{DependsOn: []string{"db"}},
			new:       models.Service{DependsOn: []string{"db", "cache"}},
			wantField: "dependsOn", wantOld: "[db]", wantNew: "[db cache]",
		},
		{
			name:      "published ports",
			old:       svcWithDocker(&models.DockerConfig{Ports: []string{"8080:80"}}),
			new:       svcWithDocker(&models.DockerConfig{Ports: []string{"9090:80"}}),
			wantField: "docker.ports", wantOld: "[8080:80]", wantNew: "[9090:80]",
		},
		{
			name:      "docker-level dependsOn",
			old:       svcWithDocker(&models.DockerConfig{DependsOn: []string{"db"}}),
			new:       svcWithDocker(&models.DockerConfig{}),
			wantField: "docker.dependsOn", wantOld: "[db]", wantNew: "[]",
		},
		{
			name:      "dockerfile path",
			old:       svcWithDocker(&models.DockerConfig{Dockerfile: "Dockerfile"}),
			new:       svcWithDocker(&models.DockerConfig{Dockerfile: "Dockerfile.dev"}),
			wantField: "docker.dockerfile", wantOld: "Dockerfile", wantNew: "Dockerfile.dev",
		},
		{
			name:      "command",
			old:       svcWithDocker(&models.DockerConfig{Command: "serve"}),
			new:       svcWithDocker(&models.DockerConfig{Command: "serve --dev"}),
			wantField: "docker.command", wantOld: "serve", wantNew: "serve --dev",
		},
		{
			// Moving a service between docker and the host changes how it
			// is started entirely, so it has to show up as one change.
			name:      "docker to host",
			old:       svcWithDocker(&models.DockerConfig{}),
			new:       models.Service{},
			wantField: "docker", wantOld: "docker config present",
			wantNew: "host execution (source.command)",
		},
		{
			name:      "host to docker",
			old:       models.Service{},
			new:       svcWithDocker(&models.DockerConfig{}),
			wantField: "docker", wantOld: "host execution (source.command)",
			wantNew: "docker config present",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := compareServiceFields("api", tc.old, tc.new)
			if len(got) != 1 {
				t.Fatalf("expected exactly one change, got %+v", got)
			}
			c := got[0]
			if c.Field != tc.wantField {
				t.Errorf("Field = %q, want %q", c.Field, tc.wantField)
			}
			if c.OldValue != tc.wantOld || c.NewValue != tc.wantNew {
				t.Errorf("values = %q → %q, want %q → %q",
					c.OldValue, c.NewValue, tc.wantOld, tc.wantNew)
			}
			if c.Name != "api" || c.Type != "service" {
				t.Errorf("change should name the service: %+v", c)
			}
		})
	}
}

// An unchanged service must produce nothing, or every up would look like
// a drift and the report would stop meaning anything.
func TestCompareServiceFieldsIdentical(t *testing.T) {
	svc := models.Service{
		Source:    models.SourceConfig{Image: "nginx", Tag: "1.25"},
		DependsOn: []string{"db"},
		Docker:    &models.DockerConfig{Ports: []string{"8080:80"}},
	}
	if got := compareServiceFields("api", svc, svc); len(got) != 0 {
		t.Errorf("identical service reported changes: %+v", got)
	}
}

// Dependencies drift the same way services do — an image or tag change
// there is what makes `up` recreate the container.
func TestCompareInfra(t *testing.T) {
	old := map[string]models.InfraEntry{
		"db":      {Inline: &models.Infra{Image: "postgres", Tag: "16"}},
		"removed": {Inline: &models.Infra{Image: "redis", Tag: "7"}},
	}
	new := map[string]models.InfraEntry{
		"db":    {Inline: &models.Infra{Image: "postgres", Tag: "17"}},
		"added": {Inline: &models.Infra{Image: "nats", Tag: "2"}},
	}

	changes := compareInfra(old, new)

	byName := map[string][]ConfigChange{}
	for _, c := range changes {
		byName[c.Name] = append(byName[c.Name], c)
	}
	if len(byName["db"]) == 0 {
		t.Error("a tag change on an existing dep must be reported")
	}
	if len(byName["added"]) == 0 || len(byName["removed"]) == 0 {
		t.Errorf("added and removed deps must both show up, got %+v", changes)
	}
}

// formatSlice feeds the human-readable report; an empty list has to read
// as empty rather than as the Go zero value.
func TestFormatSlice(t *testing.T) {
	cases := []struct {
		in   []string
		want string
	}{
		{in: nil, want: "[]"},
		{in: []string{}, want: "[]"},
		{in: []string{"a"}, want: "[a]"},
		{in: []string{"a", "b"}, want: "[a b]"},
	}
	for _, tc := range cases {
		if got := formatSlice(tc.in); got != tc.want {
			t.Errorf("formatSlice(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// Only some drift justifies recreating a container; the rest is noise the
// user should see but not pay a restart for.
func TestHasSignificantChangesAndFormatting(t *testing.T) {
	significant := []ConfigChange{{Type: "service", Name: "api", Field: "source.tag"}}
	if !HasSignificantChanges(significant) {
		t.Error("a tag change must count as significant")
	}

	cosmetic := []ConfigChange{{Type: "service", Name: "api", Field: "health"}}
	if HasSignificantChanges(cosmetic) {
		t.Error("a health-check tweak must not force a recreation")
	}

	out := FormatChanges(append(significant, cosmetic...))
	if !strings.Contains(out, "api") || !strings.Contains(out, "source.tag") {
		t.Errorf("the report must name the service and the field:\n%s", out)
	}
}
