package snapshot

import (
	"context"

	"raioz/internal/domain/interfaces"
	snapshotpkg "raioz/internal/snapshot"
)

// Compile-time check
var _ interfaces.SnapshotManager = (*ManagerImpl)(nil)

// ManagerImpl adapts the internal/snapshot package to the
// interfaces.SnapshotManager port. Behavior is unchanged from the
// underlying package; this wrapper exists so the app layer can
// program against the port instead of the concrete type — see ADR-014
// / issue 034.
type ManagerImpl struct {
	mgr *snapshotpkg.Manager
}

// NewManager builds a ManagerImpl backed by the default snapshots
// directory (~/.raioz/snapshots). baseDir empty falls through to the
// package's default.
func NewManager(baseDir string) *ManagerImpl {
	return &ManagerImpl{mgr: snapshotpkg.NewManager(baseDir)}
}

func (m *ManagerImpl) Create(
	_ context.Context, project, name string, volumes map[string]string,
) (*interfaces.Snapshot, error) {
	snap, err := m.mgr.Create(project, name, volumes)
	if err != nil {
		return nil, err
	}
	return convertSnapshot(snap), nil
}

func (m *ManagerImpl) Restore(_ context.Context, project, name string) error {
	return m.mgr.Restore(project, name)
}

func (m *ManagerImpl) List(_ context.Context, project string) ([]interfaces.Snapshot, error) {
	snaps, err := m.mgr.List(project)
	if err != nil {
		return nil, err
	}
	out := make([]interfaces.Snapshot, 0, len(snaps))
	for i := range snaps {
		out = append(out, *convertSnapshot(&snaps[i]))
	}
	return out, nil
}

func (m *ManagerImpl) Delete(_ context.Context, project, name string) error {
	return m.mgr.Delete(project, name)
}

func convertSnapshot(snap *snapshotpkg.Snapshot) *interfaces.Snapshot {
	if snap == nil {
		return nil
	}
	out := &interfaces.Snapshot{
		Name:      snap.Name,
		Project:   snap.Project,
		CreatedAt: snap.CreatedAt,
	}
	for _, v := range snap.Volumes {
		out.Volumes = append(out.Volumes, interfaces.VolumeSnapshot{
			VolumeName:  v.VolumeName,
			ServiceName: v.ServiceName,
			SizeBytes:   v.SizeBytes,
			ArchiveFile: v.ArchiveFile,
		})
	}
	return out
}
