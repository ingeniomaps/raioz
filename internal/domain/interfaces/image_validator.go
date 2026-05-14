package interfaces

import (
	"context"

	"raioz/internal/domain/models"
)

// ImageValidator covers Docker image hygiene: assert every image a
// project references can be pulled or already exists, plus sweep of
// unused images.
//
// ADR-012: one of six segregated interfaces composed by DockerRunner.
type ImageValidator interface {
	// ValidateAllImages probes every image referenced by deps (services
	// and infra) BEFORE compose generation, so raioz fails fast on a
	// typo'd image tag instead of mid-up.
	ValidateAllImages(deps *models.Deps) error

	// CleanUnusedImagesWithContext sweeps dangling and unreferenced
	// images. Honors dryRun: when true returns the candidate list
	// without removing.
	CleanUnusedImagesWithContext(ctx context.Context, dryRun bool) ([]string, error)
}
