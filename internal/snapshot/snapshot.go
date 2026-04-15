// Package snapshot manages backup and restore of Docker volumes for a project.
package snapshot

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

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

// NewManager creates a Manager. If baseDir is empty, uses ~/.raioz/snapshots.
func NewManager(baseDir string) *Manager {
	if baseDir == "" {
		home, _ := os.UserHomeDir()
		baseDir = filepath.Join(home, ".raioz", "snapshots")
	}
	return &Manager{baseDir: baseDir}
}

// Create exports all given volumes to tar.gz archives.
func (m *Manager) Create(project, name string, volumes map[string]string) (*Snapshot, error) {
	dir := filepath.Join(m.baseDir, project, name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create snapshot directory: %w", err)
	}

	snap := &Snapshot{
		Name:      name,
		Project:   project,
		CreatedAt: time.Now(),
	}

	for volumeName, serviceName := range volumes {
		archiveFile := volumeName + ".tar.gz"
		archivePath := filepath.Join(dir, archiveFile)

		if err := exportVolume(volumeName, archivePath); err != nil {
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
	data, _ := json.MarshalIndent(snap, "", "  ")
	os.WriteFile(metaPath, data, 0644)

	return snap, nil
}

// Restore imports volumes from a snapshot.
func (m *Manager) Restore(project, name string) error {
	dir := filepath.Join(m.baseDir, project, name)
	metaPath := filepath.Join(dir, "snapshot.json")

	data, err := os.ReadFile(metaPath)
	if err != nil {
		return fmt.Errorf("snapshot '%s' not found for project '%s'", name, project)
	}

	var snap Snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return fmt.Errorf("invalid snapshot metadata: %w", err)
	}

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
	dir := filepath.Join(m.baseDir, project)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
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
	dir := filepath.Join(m.baseDir, project, name)
	return os.RemoveAll(dir)
}

// exportVolume creates a tar.gz of a Docker volume's contents.
func exportVolume(volumeName, archivePath string) error {
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
func importVolume(volumeName, archivePath string) error {
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
