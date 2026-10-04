package snapshot

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestValidateName(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{"before-migration", false},
		{"v1.2_ok", false},
		{"", true},
		{"..", true},
		{"../..", true},
		{"a/b", true},
		{"a..b", true},
		{".hidden", true},
		{"with space", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := validateName("snapshot", tc.name); (err != nil) != tc.wantErr {
				t.Errorf("validateName(%q) err = %v, wantErr %v", tc.name, err, tc.wantErr)
			}
		})
	}
}

// The snapshot name becomes a directory under the store; `..` must never
// reach os.RemoveAll.
func TestDelete_RefusesPathTraversal(t *testing.T) {
	base := t.TempDir()
	keep := filepath.Join(base, "keep.txt")
	if err := os.WriteFile(keep, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(base, "project"), 0o755); err != nil {
		t.Fatal(err)
	}
	m := NewManager(base)

	for _, name := range []string{"..", "../..", "../keep.txt"} {
		if err := m.Delete("project", name); err == nil {
			t.Errorf("Delete(%q) succeeded, want it refused", name)
		}
	}
	if _, err := os.Stat(keep); err != nil {
		t.Errorf("file outside the snapshot was removed: %v", err)
	}
}

func stubResolver(t *testing.T, fn func(project, service, spec string) (string, bool, error)) {
	t.Helper()
	prev := volumeResolver
	volumeResolver = fn
	t.Cleanup(func() { volumeResolver = prev })
}

// A failed create must not leave a directory that lists as a snapshot.
func TestCreate_UnresolvedVolumeLeavesNothing(t *testing.T) {
	base := t.TempDir()
	stubResolver(t, func(string, string, string) (string, bool, error) {
		return "", true, errors.New("no Docker volume found")
	})

	_, err := NewManager(base).Create("bencha", "s1", map[string]string{"cachedata:/data": "cache"})
	if err == nil {
		t.Fatal("expected an error")
	}
	if _, statErr := os.Stat(filepath.Join(base, "bencha", "s1")); statErr == nil {
		t.Error("snapshot directory was left behind")
	}
}

// Bind mounts are not volumes: they are skipped, not handed to docker as a
// volume name.
func TestCreate_SkipsBindMounts(t *testing.T) {
	base := t.TempDir()
	var asked []string
	stubResolver(t, func(_, _, spec string) (string, bool, error) {
		asked = append(asked, spec)
		return "", false, nil
	})

	snap, err := NewManager(base).Create("bencha", "s1", map[string]string{"./conf:/etc/conf": "cache"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if len(snap.Volumes) != 0 {
		t.Errorf("Volumes = %v, want none", snap.Volumes)
	}
	if len(asked) != 1 {
		t.Errorf("resolver asked for %v", asked)
	}
}

func TestResolveVolume_BindMounts(t *testing.T) {
	for _, spec := range []string{"./data:/data", "/var/lib/x:/data", "~/x:/data", ":/data"} {
		if _, ok, err := resolveVolume("bencha", "cache", spec); ok || err != nil {
			t.Errorf("resolveVolume(%q) ok=%v err=%v, want a skipped bind mount", spec, ok, err)
		}
	}
}
