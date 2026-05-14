package interfaces

import (
	"context"
	"time"
)

// Snapshot is the metadata raioz records for a saved volume backup. It
// lives in the domain layer so callers can pass it around without
// importing internal/snapshot.
type Snapshot struct {
	Name      string
	Project   string
	CreatedAt time.Time
	Volumes   []VolumeSnapshot
}

// VolumeSnapshot describes one Docker volume inside a Snapshot.
type VolumeSnapshot struct {
	VolumeName  string
	ServiceName string
	SizeBytes   int64
	ArchiveFile string
}

// SnapshotManager is the port covering volume backup/restore. Issue
// 034 introduced it so the `raioz snapshot` CLI commands could run
// through the use-case layer like every other command. Tests stub
// this interface to assert lifecycle without booting Docker.
type SnapshotManager interface {
	// Create saves `volumes` (volumeName → serviceName) to a tarball
	// set tagged with `name` under the manager's snapshot root for
	// `project`. Returns the metadata that was persisted.
	Create(
		ctx context.Context, project, name string,
		volumes map[string]string,
	) (*Snapshot, error)

	// Restore replays `name` over the project's named volumes.
	// Existing volume contents are overwritten in place.
	Restore(ctx context.Context, project, name string) error

	// List returns every snapshot raioz knows about for `project`,
	// newest-first ordering not guaranteed (callers sort as needed).
	List(ctx context.Context, project string) ([]Snapshot, error)

	// Delete removes `name` from the project's snapshot directory.
	// Idempotent — missing snapshots return nil.
	Delete(ctx context.Context, project, name string) error
}
