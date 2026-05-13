package interfaces

import (
	"context"

	"raioz/internal/domain/models"
)

// VolumeManager covers Docker volume operations: ensure named volumes
// exist, sweep unused, derive named-volume sets from a project's deps,
// and rewrite bind-mount paths to absolute.
//
// ADR-012: one of six segregated interfaces composed by DockerRunner.
type VolumeManager interface {
	EnsureVolumeWithContext(ctx context.Context, name string) error
	RemoveVolumeWithContext(ctx context.Context, name string) error

	GetVolumeProjects(volumeName, baseDir string) ([]string, error)
	ExtractNamedVolumes(volumes []string) ([]string, error)
	ResolveRelativeVolumes(volumes []string, projectDir string) ([]string, error)

	CleanUnusedVolumesWithContext(
		ctx context.Context, dryRun, force bool,
	) ([]string, error)

	BuildServiceVolumesMap(deps *models.Deps) (map[string]ServiceVolumes, error)
	DetectSharedVolumes(services map[string]ServiceVolumes) map[string][]string
}
