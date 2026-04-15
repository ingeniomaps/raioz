package upcase

import (
	"context"
	"errors"
	"testing"

	"raioz/internal/config"
	"raioz/internal/detect"
	"raioz/internal/docker"
)

// resetExposedPortCacheTestHook lives in the docker package; we can't reach
// its internals from outside. Seed the cache with public image references
// to drive the enrichment code paths deterministically.
func seedExposedPort(image string, port int, err error) {
	docker.SetExposedPortCacheForTest(image, port, err)
}

func TestEnrichDetectionsWithExposedPorts_BackfillsImageDep(t *testing.T) {
	const image = "postgres:99-exposed-test"
	seedExposedPort(image, 5432, nil)

	deps := &config.Deps{
		Infra: map[string]config.InfraEntry{
			"postgres": {Inline: &config.Infra{Image: "postgres", Tag: "99-exposed-test"}},
		},
	}
	detections := DetectionMap{
		"postgres": {Runtime: detect.RuntimeImage, Port: 0},
	}

	enrichDetectionsWithExposedPorts(context.Background(), deps, detections)

	if got := detections["postgres"].Port; got != 5432 {
		t.Errorf("Port = %d, want 5432", got)
	}
}

func TestEnrichDetectionsWithExposedPorts_SkipsWhenPortKnown(t *testing.T) {
	const image = "redis:99-exposed-test"
	// Deliberately seed with a different port — we should NOT overwrite.
	seedExposedPort(image, 7777, nil)

	deps := &config.Deps{
		Infra: map[string]config.InfraEntry{
			"redis": {Inline: &config.Infra{Image: "redis", Tag: "99-exposed-test"}},
		},
	}
	detections := DetectionMap{
		"redis": {Runtime: detect.RuntimeImage, Port: 6379},
	}

	enrichDetectionsWithExposedPorts(context.Background(), deps, detections)

	if got := detections["redis"].Port; got != 6379 {
		t.Errorf("Port = %d, want 6379 (existing value preserved)", got)
	}
}

func TestEnrichDetectionsWithExposedPorts_SilentOnLookupFailure(t *testing.T) {
	const image = "missing:99-exposed-test"
	seedExposedPort(image, 0, errors.New("no such image"))

	deps := &config.Deps{
		Infra: map[string]config.InfraEntry{
			"missing": {Inline: &config.Infra{Image: "missing", Tag: "99-exposed-test"}},
		},
	}
	detections := DetectionMap{
		"missing": {Runtime: detect.RuntimeImage, Port: 0},
	}

	enrichDetectionsWithExposedPorts(context.Background(), deps, detections)

	if got := detections["missing"].Port; got != 0 {
		t.Errorf("Port = %d, want 0 (fallback preserved on lookup error)", got)
	}
}
