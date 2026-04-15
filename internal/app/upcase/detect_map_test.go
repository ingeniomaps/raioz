package upcase

import (
	"testing"

	"raioz/internal/config"
	"raioz/internal/detect"
)

// --- BuildDetectionMap -------------------------------------------------------

func TestBuildDetectionMapServicesWithPath(t *testing.T) {
	deps := &config.Deps{
		Services: map[string]config.Service{
			"api": {Source: config.SourceConfig{Path: "/tmp/api", Command: "go run ."}},
		},
		Infra: map[string]config.InfraEntry{},
	}

	results := BuildDetectionMap(deps)
	if _, ok := results["api"]; !ok {
		t.Error("expected api in detection map")
	}
}

func TestBuildDetectionMapServicesWithCommand(t *testing.T) {
	deps := &config.Deps{
		Services: map[string]config.Service{
			"worker": {Source: config.SourceConfig{Command: "node worker.js"}},
		},
		Infra: map[string]config.InfraEntry{},
	}

	results := BuildDetectionMap(deps)
	if _, ok := results["worker"]; !ok {
		t.Error("expected worker in detection map (has command)")
	}
}

func TestBuildDetectionMapServicesWithComposeFiles(t *testing.T) {
	deps := &config.Deps{
		Services: map[string]config.Service{
			"web": {Source: config.SourceConfig{ComposeFiles: []string{"docker-compose.yml"}}},
		},
		Infra: map[string]config.InfraEntry{},
	}

	results := BuildDetectionMap(deps)
	if _, ok := results["web"]; !ok {
		t.Error("expected web in detection map (has composeFiles)")
	}
}

func TestBuildDetectionMapSkipsEmptyService(t *testing.T) {
	deps := &config.Deps{
		Services: map[string]config.Service{
			"empty": {Source: config.SourceConfig{}},
		},
		Infra: map[string]config.InfraEntry{},
	}

	results := BuildDetectionMap(deps)
	if _, ok := results["empty"]; ok {
		t.Error("service with no path, command, or composeFiles should be skipped")
	}
}

func TestBuildDetectionMapInfraWithTag(t *testing.T) {
	deps := &config.Deps{
		Services: map[string]config.Service{},
		Infra: map[string]config.InfraEntry{
			"postgres": {Inline: &config.Infra{Image: "postgres", Tag: "16"}},
		},
	}

	results := BuildDetectionMap(deps)
	r, ok := results["postgres"]
	if !ok {
		t.Fatal("expected postgres in detection map")
	}
	if r.Runtime != detect.RuntimeImage {
		t.Errorf("runtime = %q, want image", r.Runtime)
	}
}

func TestBuildDetectionMapInfraWithoutTag(t *testing.T) {
	deps := &config.Deps{
		Services: map[string]config.Service{},
		Infra: map[string]config.InfraEntry{
			"redis": {Inline: &config.Infra{Image: "redis"}},
		},
	}

	results := BuildDetectionMap(deps)
	r, ok := results["redis"]
	if !ok {
		t.Fatal("expected redis in detection map")
	}
	if r.Runtime != detect.RuntimeImage {
		t.Errorf("runtime = %q, want image", r.Runtime)
	}
}

func TestBuildDetectionMapInfraNilInline(t *testing.T) {
	deps := &config.Deps{
		Services: map[string]config.Service{},
		Infra: map[string]config.InfraEntry{
			"ext": {Path: "/some/path"},
		},
	}

	results := BuildDetectionMap(deps)
	r, ok := results["ext"]
	if !ok {
		t.Fatal("expected ext in detection map")
	}
	if r.Runtime != detect.RuntimeImage {
		t.Errorf("runtime = %q, want image", r.Runtime)
	}
}

func TestBuildDetectionMapMixed(t *testing.T) {
	deps := &config.Deps{
		Services: map[string]config.Service{
			"api":   {Source: config.SourceConfig{Path: "/tmp/api", Command: "go run ."}},
			"empty": {Source: config.SourceConfig{}},
		},
		Infra: map[string]config.InfraEntry{
			"db": {Inline: &config.Infra{Image: "postgres", Tag: "16"}},
		},
	}

	results := BuildDetectionMap(deps)
	if len(results) != 2 {
		t.Errorf("expected 2 results (api + db), got %d", len(results))
	}
	if _, ok := results["api"]; !ok {
		t.Error("missing api")
	}
	if _, ok := results["db"]; !ok {
		t.Error("missing db")
	}
	if _, ok := results["empty"]; ok {
		t.Error("empty should be skipped")
	}
}
