package snapshot

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	m := NewManager("/tmp/test-snapshots")
	if m.baseDir != "/tmp/test-snapshots" {
		t.Errorf("expected /tmp/test-snapshots, got %s", m.baseDir)
	}
}

func TestNewManager_Default(t *testing.T) {
	m := NewManager("")
	if m.baseDir == "" {
		t.Error("expected non-empty default baseDir")
	}
}

func TestList_EmptyProject(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	snapshots, err := m.List("nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(snapshots) != 0 {
		t.Errorf("expected 0 snapshots, got %d", len(snapshots))
	}
}

func TestDelete_Nonexistent(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	// Should not error on nonexistent snapshot
	err := m.Delete("project", "nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRestore_Nonexistent(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	err := m.Restore("project", "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent snapshot")
	}
}

// writeFakeSnapshot creates a valid snapshot metadata file (without real volumes)
// so we can test the List and Restore metadata-loading paths.
func writeFakeSnapshot(t *testing.T, baseDir, project, name string, volumes []VolumeSnapshot) string {
	t.Helper()
	dir := filepath.Join(baseDir, project, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	snap := Snapshot{
		Name:      name,
		Project:   project,
		CreatedAt: time.Now(),
		Volumes:   volumes,
	}
	data, _ := json.MarshalIndent(snap, "", "  ")
	metaPath := filepath.Join(dir, "snapshot.json")
	if err := os.WriteFile(metaPath, data, 0o644); err != nil {
		t.Fatalf("write meta: %v", err)
	}
	return dir
}

func TestList_WithSnapshots(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	writeFakeSnapshot(t, dir, "proj1", "snap1", nil)
	writeFakeSnapshot(t, dir, "proj1", "snap2", nil)

	snapshots, err := m.List("proj1")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(snapshots) != 2 {
		t.Errorf("expected 2 snapshots, got %d", len(snapshots))
	}
}

func TestList_SkipsInvalidMetadata(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	// Valid snapshot
	writeFakeSnapshot(t, dir, "proj1", "good", nil)

	// Directory without metadata
	os.MkdirAll(filepath.Join(dir, "proj1", "no-meta"), 0o755)

	// Invalid JSON metadata
	invalidDir := filepath.Join(dir, "proj1", "invalid")
	os.MkdirAll(invalidDir, 0o755)
	os.WriteFile(filepath.Join(invalidDir, "snapshot.json"), []byte("not-json"), 0o644)

	snapshots, err := m.List("proj1")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	// Should only return the valid one
	if len(snapshots) != 1 {
		t.Errorf("expected 1 valid snapshot, got %d", len(snapshots))
	}
}

func TestList_IgnoresFiles(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	projDir := filepath.Join(dir, "proj1")
	os.MkdirAll(projDir, 0o755)

	// A file (not a directory) in project dir should be ignored
	os.WriteFile(filepath.Join(projDir, "random.txt"), []byte("x"), 0o644)

	snapshots, err := m.List("proj1")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(snapshots) != 0 {
		t.Errorf("expected 0, got %d", len(snapshots))
	}
}

func TestDelete_RemovesDirectory(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	snapDir := writeFakeSnapshot(t, dir, "proj1", "todelete", nil)

	if err := m.Delete("proj1", "todelete"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if _, err := os.Stat(snapDir); !os.IsNotExist(err) {
		t.Error("expected snapshot dir to be removed")
	}
}

func TestRestore_InvalidMetadata(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	// Create dir with invalid JSON
	badDir := filepath.Join(dir, "proj1", "bad")
	os.MkdirAll(badDir, 0o755)
	os.WriteFile(filepath.Join(badDir, "snapshot.json"), []byte("not json"), 0o644)

	err := m.Restore("proj1", "bad")
	if err == nil {
		t.Error("expected error for invalid metadata")
	}
}

func TestCreate_EmptyVolumes(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	// Empty volumes map skips the exportVolume loop entirely —
	// so Create can complete without Docker.
	snap, err := m.Create("proj1", "snap1", map[string]string{})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if snap.Name != "snap1" || snap.Project != "proj1" {
		t.Errorf("unexpected snapshot: %+v", snap)
	}
	if len(snap.Volumes) != 0 {
		t.Errorf("expected 0 volumes, got %d", len(snap.Volumes))
	}

	// Metadata file should have been written
	metaPath := filepath.Join(dir, "proj1", "snap1", "snapshot.json")
	if _, err := os.Stat(metaPath); err != nil {
		t.Errorf("expected metadata file: %v", err)
	}

	// And List should return it
	snaps, err := m.List("proj1")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(snaps) != 1 {
		t.Errorf("expected 1 snapshot, got %d", len(snaps))
	}
}

func TestCreate_MkdirError(t *testing.T) {
	// Use a file as baseDir to force MkdirAll to fail
	dir := t.TempDir()
	baseFile := filepath.Join(dir, "not-a-dir")
	if err := os.WriteFile(baseFile, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := NewManager(baseFile)
	_, err := m.Create("proj", "snap", map[string]string{})
	if err == nil {
		t.Error("expected error when baseDir is a file")
	}
}

func TestCreate_WithDockerDependentVolumes(t *testing.T) {
	// Creating a snapshot with actual volumes requires docker+alpine.
	// Skip here — this path is exercised manually.
	t.Skip("requires docker runtime (exportVolume); covered by e2e")
}

func TestCreate_ExportVolumeFails(t *testing.T) {
	// Pass a bogus volume name so exportVolume (docker run) fails fast.
	// This exercises the error-return branch inside Create's loop
	// without requiring a live docker daemon to produce an archive.
	// Docker not being available also causes a fast failure.
	dir := t.TempDir()
	m := NewManager(dir)

	_, err := m.Create("proj", "snap", map[string]string{
		"definitely-not-a-real-volume-raioz-test": "svc",
	})
	if err == nil {
		t.Skip("unexpected success — docker may have created a volume; skipping")
	}
}

func TestRestore_NoVolumes(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	// Create a valid empty snapshot (no volumes), then restore it.
	// This exercises Restore's metadata parse + empty loop path.
	writeFakeSnapshot(t, dir, "proj1", "empty", nil)

	if err := m.Restore("proj1", "empty"); err != nil {
		t.Errorf("expected success on empty-volume snapshot, got: %v", err)
	}
}

func TestRestore_WithDockerDependentVolumes(t *testing.T) {
	// importVolume requires docker+alpine.
	t.Skip("requires docker runtime (importVolume); covered by e2e")
}

func TestRestore_ImportVolumeFails(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	// Write a snapshot with a volume whose archive file does not exist.
	// importVolume will fail fast (either missing mount source or docker error).
	vols := []VolumeSnapshot{
		{
			VolumeName:  "definitely-not-a-real-volume-raioz-test",
			ServiceName: "svc",
			SizeBytes:   0,
			ArchiveFile: "ghost.tar.gz",
		},
	}
	writeFakeSnapshot(t, dir, "proj", "snap", vols)

	err := m.Restore("proj", "snap")
	if err == nil {
		t.Skip("unexpected success — docker may have accepted missing archive; skipping")
	}
}

func TestList_NonexistentBaseDir(t *testing.T) {
	m := NewManager(filepath.Join(t.TempDir(), "never-existed"))
	snaps, err := m.List("anything")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if len(snaps) != 0 {
		t.Errorf("expected empty, got %d", len(snaps))
	}
}

func TestSnapshotStructSerialization(t *testing.T) {
	snap := Snapshot{
		Name:      "test",
		Project:   "proj",
		CreatedAt: time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC),
		Volumes: []VolumeSnapshot{
			{
				VolumeName:  "data",
				ServiceName: "api",
				SizeBytes:   1024,
				ArchiveFile: "data.tar.gz",
			},
		},
	}

	data, err := json.Marshal(snap)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var got Snapshot
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if got.Name != "test" || len(got.Volumes) != 1 {
		t.Errorf("round-trip mismatch: %+v", got)
	}
}
