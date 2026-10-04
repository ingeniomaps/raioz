// Package snapshot manages backup and restore of Docker volumes for a project.
package snapshot

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"raioz/internal/i18n"
	"raioz/internal/naming"
	"raioz/internal/runtime"
)

// Snapshot holds metadata about a saved volume snapshot.
type Snapshot struct {
	Name      string           `json:"name"`
	Project   string           `json:"project"`
	CreatedAt time.Time        `json:"createdAt"`
	Volumes   []VolumeSnapshot `json:"volumes"`
}

// VolumeSnapshot represents one volume in a snapshot.
type VolumeSnapshot struct {
	VolumeName  string `json:"volumeName"`
	ServiceName string `json:"serviceName"`
	SizeBytes   int64  `json:"sizeBytes"`
	ArchiveFile string `json:"archiveFile"`
}

// Manager handles snapshot operations.
type Manager struct {
	baseDir string // ~/.raioz/snapshots
}

// NewManager creates a Manager. An empty baseDir means the snapshots
// directory under the raioz state dir (ADR-022).
func NewManager(baseDir string) *Manager {
	if baseDir == "" {
		baseDir = filepath.Join(naming.RaiozStateDir(), "snapshots")
	}
	return &Manager{baseDir: baseDir}
}

// legacyBaseDir is where snapshots lived before they followed the state
// dir. Snapshots taken there are still listed, restored and deleted.
func legacyBaseDir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".raioz", "snapshots")
}

// snapshotDir returns the directory of an existing snapshot, looking in
// the legacy store when the current one does not have it. A snapshot that
// exists in neither resolves to the current store.
func (m *Manager) snapshotDir(project, name string) string {
	dir := filepath.Join(m.baseDir, project, name)
	if _, err := os.Stat(dir); err == nil {
		return dir
	}
	if legacy := legacyBaseDir(); legacy != "" && legacy != m.baseDir {
		old := filepath.Join(legacy, project, name)
		if _, err := os.Stat(old); err == nil {
			return old
		}
	}
	return dir
}

// Create exports all given volumes to tar.gz archives.
func (m *Manager) Create(project, name string, volumes map[string]string) (*Snapshot, error) {
	if err := validateName("snapshot", name); err != nil {
		return nil, err
	}

	// Resolve every volume before writing anything: a snapshot that fails
	// halfway must not leave a directory that looks like a snapshot.
	type target struct{ volume, service, archive string }
	var targets []target
	for spec, serviceName := range volumes {
		volumeName, ok, err := volumeResolver(project, serviceName, spec)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		targets = append(targets, target{volumeName, serviceName, volumeName + ".tar.gz"})
	}

	dir := filepath.Join(m.baseDir, project, name)
	// A snapshot is what you go back to; replacing one silently loses it.
	if _, err := os.Stat(filepath.Join(dir, "snapshot.json")); err == nil {
		return nil, fmt.Errorf("%s", i18n.T("error.snapshot_exists", name))
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create snapshot directory: %w", err)
	}

	snap := &Snapshot{
		Name:      name,
		Project:   project,
		CreatedAt: time.Now(),
	}

	// Copy with the engines stopped, for the reason restore does: a
	// database holds part of its state in memory, and files read under it
	// are a copy of a moment that never existed on disk.
	using := make([]VolumeSnapshot, 0, len(targets))
	for _, t := range targets {
		using = append(using, VolumeSnapshot{VolumeName: t.volume})
	}
	resume, err := quiesce(using, "snapshot.stopping_for_create")
	if err != nil {
		_ = os.RemoveAll(dir)
		return nil, fmt.Errorf("failed to stop the containers using the volumes: %w", err)
	}
	defer resume()

	for _, t := range targets {
		volumeName, serviceName, archiveFile := t.volume, t.service, t.archive
		archivePath := filepath.Join(dir, archiveFile)

		if err := exportVolume(volumeName, archivePath); err != nil {
			_ = os.RemoveAll(dir)
			return nil, fmt.Errorf("failed to export volume %s: %w", volumeName, err)
		}

		info, _ := os.Stat(archivePath)
		var size int64
		if info != nil {
			size = info.Size()
		}

		snap.Volumes = append(snap.Volumes, VolumeSnapshot{
			VolumeName:  volumeName,
			ServiceName: serviceName,
			SizeBytes:   size,
			ArchiveFile: archiveFile,
		})
	}

	// Save metadata
	metaPath := filepath.Join(dir, "snapshot.json")
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal snapshot metadata: %w", err)
	}
	if err := os.WriteFile(metaPath, data, 0644); err != nil {
		return nil, fmt.Errorf("failed to write snapshot metadata: %w", err)
	}

	return snap, nil
}

// Restore imports volumes from a snapshot.
func (m *Manager) Restore(project, name string) error {
	if err := validateName("snapshot", name); err != nil {
		return err
	}
	dir := m.snapshotDir(project, name)
	metaPath := filepath.Join(dir, "snapshot.json")

	data, err := os.ReadFile(metaPath)
	if err != nil {
		return fmt.Errorf("%s", i18n.T("error.snapshot_not_found", name, project))
	}

	var snap Snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return fmt.Errorf("invalid snapshot metadata: %w", err)
	}

	// Whatever mounts these volumes is stopped for the duration and started
	// again afterwards, restore succeeded or not.
	resume, err := quiesce(snap.Volumes, "snapshot.stopping_for_restore")
	if err != nil {
		return fmt.Errorf("failed to stop the containers using the volumes: %w", err)
	}
	defer resume()

	for _, vol := range snap.Volumes {
		archivePath := filepath.Join(dir, vol.ArchiveFile)
		if err := importVolume(vol.VolumeName, archivePath); err != nil {
			return fmt.Errorf("failed to restore volume %s: %w", vol.VolumeName, err)
		}
	}

	return nil
}

// List returns all snapshots for a project.
func (m *Manager) List(project string) ([]Snapshot, error) {
	snapshots, err := listDir(filepath.Join(m.baseDir, project))
	if err != nil {
		return nil, err
	}
	if legacy := legacyBaseDir(); legacy != "" && legacy != m.baseDir {
		seen := map[string]bool{}
		for _, s := range snapshots {
			seen[s.Name] = true
		}
		old, _ := listDir(filepath.Join(legacy, project))
		for _, s := range old {
			if !seen[s.Name] {
				snapshots = append(snapshots, s)
			}
		}
	}
	return snapshots, nil
}

func listDir(dir string) ([]Snapshot, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read snapshot dir %q: %w", dir, err)
	}

	var snapshots []Snapshot
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		metaPath := filepath.Join(dir, entry.Name(), "snapshot.json")
		data, err := os.ReadFile(metaPath)
		if err != nil {
			continue
		}
		var snap Snapshot
		if json.Unmarshal(data, &snap) == nil {
			snapshots = append(snapshots, snap)
		}
	}
	return snapshots, nil
}

// Delete removes a snapshot and frees disk space.
func (m *Manager) Delete(project, name string) error {
	if err := validateName("snapshot", name); err != nil {
		return err
	}
	dir := m.snapshotDir(project, name)
	if _, err := os.Stat(dir); err != nil {
		return fmt.Errorf("%s", i18n.T("error.snapshot_not_found", name, project))
	}
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("remove snapshot dir %q: %w", dir, err)
	}
	return nil
}

// exportVolume creates a tar.gz of a Docker volume's contents.
// exportVolume is a package var for the same reason importVolume is: a test
// that asks the daemon to mount a volume creates it for real.
var exportVolume = exportVolumeWithDocker

func exportVolumeWithDocker(volumeName, archivePath string) error {
	cmd := exec.Command(runtime.Binary(), "run", "--rm",
		"-v", volumeName+":/data:ro",
		"-v", filepath.Dir(archivePath)+":/backup",
		"alpine",
		"tar", "czf", "/backup/"+filepath.Base(archivePath), "-C", "/data", ".",
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%w: %s", err, string(output))
	}
	return nil
}

// importVolume restores a tar.gz into a Docker volume.
// importVolume is a package var so tests can fail it without asking the
// daemon to mount (and thereby create) a volume.
var importVolume = importVolumeWithDocker

func importVolumeWithDocker(volumeName, archivePath string) error {
	cmd := exec.Command(runtime.Binary(), "run", "--rm",
		"-v", volumeName+":/data",
		"-v", filepath.Dir(archivePath)+":/backup:ro",
		"alpine",
		"sh", "-c", "rm -rf /data/* /data/.[!.]* && tar xzf /backup/"+filepath.Base(archivePath)+" -C /data",
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%w: %s", err, string(output))
	}
	return nil
}
