package production

import (
	"strings"
	"testing"

	"raioz/internal/domain/models"
)

func localDeps(services map[string]models.Service, infra map[string]models.InfraEntry) *models.Deps {
	return &models.Deps{Services: services, Infra: infra}
}

// `raioz compare` used to panic here: a service declared with `command:`
// and no `ports:` carries no docker block, and the port comparison
// dereferenced it. That is the recommended shape for Next/Vite services,
// so the crash hit the common case.
func TestCompareConfigsServiceWithoutDockerBlock(t *testing.T) {
	local := localDeps(map[string]models.Service{
		"api": {Source: models.SourceConfig{Kind: "local", Command: "npm run dev"}},
	}, nil)
	prod := &ProductionConfig{Services: map[string]ProductionService{
		"api": {Image: "registry/api:1.2.3", Ports: []string{"8080:8080"}},
	}}

	result := CompareConfigs(local, prod)

	if len(result.ServiceDifferences) != 1 {
		t.Fatalf("expected one difference, got %+v", result.ServiceDifferences)
	}
	diff := result.ServiceDifferences[0]
	if diff.PortMismatch == nil {
		t.Error("production publishes a port the local service does not; expected a port mismatch")
	}
}
func TestCompareConfigsServicePresence(t *testing.T) {
	local := localDeps(map[string]models.Service{
		"only-local": {Source: models.SourceConfig{Kind: "image", Image: "nginx", Tag: "alpine"}},
	}, nil)
	prod := &ProductionConfig{Services: map[string]ProductionService{
		"only-prod": {Image: "nginx:alpine"},
	}}

	result := CompareConfigs(local, prod)

	if len(result.ServiceDifferences) != 2 {
		t.Fatalf("expected both sides reported, got %+v", result.ServiceDifferences)
	}
	byName := map[string]ServiceDifference{}
	for _, d := range result.ServiceDifferences {
		byName[d.ServiceName] = d
	}
	if !byName["only-local"].InLocalOnly || byName["only-local"].Severity != "warning" {
		t.Errorf("local-only service = %+v; want InLocalOnly with warning", byName["only-local"])
	}
	// Something running in production but absent locally is informational:
	// the local config is not meant to mirror prod one-to-one.
	if !byName["only-prod"].InProductionOnly || byName["only-prod"].Severity != "info" {
		t.Errorf("prod-only service = %+v; want InProductionOnly with info", byName["only-prod"])
	}
}

// Severity is what the user scans for, so each kind of drift has to map
// to the level the command promises: a tag drift warns, a dependency
// drift is an error.
func TestCompareConfigsSeverity(t *testing.T) {
	cases := []struct {
		name         string
		local        models.Service
		prod         ProductionService
		wantSeverity string
		check        func(t *testing.T, d ServiceDifference)
	}{
		{
			name: "tag drift warns",
			local: models.Service{
				Source: models.SourceConfig{Kind: "image", Image: "nginx", Tag: "1.25"},
				Docker: &models.DockerConfig{},
			},
			prod:         ProductionService{Image: "nginx:1.27"},
			wantSeverity: "warning",
			check: func(t *testing.T, d ServiceDifference) {
				if d.ImageMismatch == nil || d.ImageMismatch.ProdTag != "1.27" {
					t.Errorf("image mismatch = %+v, want prod tag 1.27", d.ImageMismatch)
				}
			},
		},
		{
			name: "dependency drift is an error",
			local: models.Service{
				Source: models.SourceConfig{Kind: "image", Image: "nginx", Tag: "1.25"},
				Docker: &models.DockerConfig{DependsOn: []string{"db"}},
			},
			prod:         ProductionService{Image: "nginx:1.25", DependsOn: []string{"db", "cache"}},
			wantSeverity: "error",
			check: func(t *testing.T, d ServiceDifference) {
				if d.DependsMismatch == nil {
					t.Error("expected a depends_on mismatch")
				}
			},
		},
		{
			name: "volume drift stays informational",
			local: models.Service{
				Source: models.SourceConfig{Kind: "image", Image: "nginx", Tag: "1.25"},
				Docker: &models.DockerConfig{Volumes: []string{"./src:/app"}},
			},
			prod:         ProductionService{Image: "nginx:1.25"},
			wantSeverity: "info",
			check: func(t *testing.T, d ServiceDifference) {
				if d.VolumeMismatch == nil {
					t.Error("expected a volume mismatch")
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := CompareConfigs(
				localDeps(map[string]models.Service{"api": tc.local}, nil),
				&ProductionConfig{Services: map[string]ProductionService{"api": tc.prod}},
			)
			if len(result.ServiceDifferences) != 1 {
				t.Fatalf("expected one difference, got %+v", result.ServiceDifferences)
			}
			got := result.ServiceDifferences[0]
			if got.Severity != tc.wantSeverity {
				t.Errorf("Severity = %q, want %q", got.Severity, tc.wantSeverity)
			}
			tc.check(t, got)
		})
	}
}

// An identical service must produce no entry at all — otherwise the
// report is noise and the drift gets lost in it.
func TestCompareConfigsIdenticalServiceIsSilent(t *testing.T) {
	svc := models.Service{
		Source: models.SourceConfig{Kind: "image", Image: "nginx", Tag: "1.25"},
		Docker: &models.DockerConfig{Ports: []string{"8080:8080"}},
	}
	result := CompareConfigs(
		localDeps(map[string]models.Service{"api": svc}, nil),
		&ProductionConfig{Services: map[string]ProductionService{
			"api": {Image: "nginx:1.25", Ports: []string{"8080:8080"}},
		}},
	)
	if len(result.ServiceDifferences) != 0 {
		t.Errorf("identical service reported as different: %+v", result.ServiceDifferences)
	}
}

func TestCompareConfigsInfra(t *testing.T) {
	local := localDeps(nil, map[string]models.InfraEntry{
		"db":      {Inline: &models.Infra{Image: "postgres", Tag: "16"}},
		"missing": {Inline: &models.Infra{Image: "redis", Tag: "7"}},
		// Path-based fragments have no inline spec to compare against.
		"external": {Path: "./infra/kafka.yml"},
	})
	prod := &ProductionConfig{Services: map[string]ProductionService{
		"db": {Image: "postgres:17"},
	}}

	result := CompareConfigs(local, prod)

	byName := map[string]InfraDifference{}
	for _, d := range result.InfraDifferences {
		byName[d.InfraName] = d
	}
	if d, ok := byName["db"]; !ok || d.ImageMismatch == nil || d.Severity != "warning" {
		t.Errorf("db = %+v, want an image mismatch at warning", d)
	}
	if d, ok := byName["missing"]; !ok || !d.InLocalOnly {
		t.Errorf("missing = %+v, want InLocalOnly", d)
	}
	if _, ok := byName["external"]; ok {
		t.Error("path-based infra has no inline spec and must not be compared")
	}
}

// The formatter is what the user actually reads, so the differences have
// to reach the output rather than being counted and dropped.
func TestFormatComparisonResult(t *testing.T) {
	empty := FormatComparisonResult(&ComparisonResult{})
	if empty == "" {
		t.Error("an empty result must still say something")
	}

	result := CompareConfigs(
		localDeps(map[string]models.Service{
			"api": {
				Source: models.SourceConfig{Kind: "image", Image: "nginx", Tag: "1.25"},
				Docker: &models.DockerConfig{},
			},
		}, nil),
		&ProductionConfig{Services: map[string]ProductionService{"api": {Image: "nginx:1.27"}}},
	)
	out := FormatComparisonResult(result)
	if !strings.Contains(out, "api") {
		t.Errorf("formatted output must name the drifting service:\n%s", out)
	}
	if !strings.Contains(out, "1.27") {
		t.Errorf("formatted output must show the production value:\n%s", out)
	}
}
