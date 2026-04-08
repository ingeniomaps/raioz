package link

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsLinked(t *testing.T) {
	t.Run("path does not exist", func(t *testing.T) {
		tmpDir := t.TempDir()
		isLinked, target, err := IsLinked(filepath.Join(tmpDir, "nonexistent"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if isLinked {
			t.Error("expected not linked")
		}
		if target != "" {
			t.Error("expected empty target")
		}
	})

	t.Run("path is a directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		dirPath := filepath.Join(tmpDir, "service-dir")
		os.MkdirAll(dirPath, 0755)

		isLinked, target, err := IsLinked(dirPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if isLinked {
			t.Error("expected not linked")
		}
		if target != "" {
			t.Error("expected empty target")
		}
	})

	t.Run("path is a symlink", func(t *testing.T) {
		tmpDir := t.TempDir()
		externalPath := filepath.Join(tmpDir, "external")
		os.MkdirAll(externalPath, 0755)

		linkPath := filepath.Join(tmpDir, "link-abs")
		absExternal, _ := filepath.Abs(externalPath)
		os.Symlink(absExternal, linkPath)

		isLinked, target, err := IsLinked(linkPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !isLinked {
			t.Error("expected linked")
		}
		if target == "" {
			t.Error("expected non-empty target")
		}
		if !filepath.IsAbs(target) {
			t.Errorf("expected absolute target, got %s", target)
		}
	})

	t.Run("path is a relative symlink", func(t *testing.T) {
		tmpDir := t.TempDir()
		externalPath := filepath.Join(tmpDir, "ext-rel")
		os.MkdirAll(externalPath, 0755)

		linkPath := filepath.Join(tmpDir, "link-rel")
		os.Symlink("ext-rel", linkPath)

		isLinked, target, err := IsLinked(linkPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !isLinked {
			t.Error("expected linked")
		}
		if !filepath.IsAbs(target) {
			t.Errorf("expected absolute target, got %s", target)
		}
	})
}

func TestCreateLink(t *testing.T) {
	t.Run("create new link", func(t *testing.T) {
		tmpDir := t.TempDir()
		externalPath := filepath.Join(tmpDir, "external")
		os.MkdirAll(externalPath, 0755)
		servicePath := filepath.Join(tmpDir, "svc")

		err := CreateLink(servicePath, externalPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		isLinked, target, _ := IsLinked(servicePath)
		if !isLinked {
			t.Error("expected symlink to be created")
		}
		if target == "" {
			t.Error("expected non-empty target")
		}
	})

	t.Run("external path does not exist", func(t *testing.T) {
		tmpDir := t.TempDir()
		err := CreateLink(filepath.Join(tmpDir, "svc"), filepath.Join(tmpDir, "ghost"))
		if err == nil {
			t.Error("expected error for non-existent external path")
		}
	})

	t.Run("external path is not a directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "file.txt")
		os.WriteFile(filePath, []byte("test"), 0644)

		err := CreateLink(filepath.Join(tmpDir, "svc"), filePath)
		if err == nil {
			t.Error("expected error for file (not directory)")
		}
	})

	t.Run("service path already exists as directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		externalPath := filepath.Join(tmpDir, "external")
		os.MkdirAll(externalPath, 0755)
		servicePath := filepath.Join(tmpDir, "svc-dir")
		os.MkdirAll(servicePath, 0755)

		err := CreateLink(servicePath, externalPath)
		if err == nil {
			t.Error("expected error when service path exists as directory")
		}
	})

	t.Run("same target is no-op", func(t *testing.T) {
		tmpDir := t.TempDir()
		externalPath := filepath.Join(tmpDir, "external")
		os.MkdirAll(externalPath, 0755)
		servicePath := filepath.Join(tmpDir, "svc-same")
		absExt, _ := filepath.Abs(externalPath)
		os.Symlink(absExt, servicePath)

		err := CreateLink(servicePath, externalPath)
		if err != nil {
			t.Fatalf("expected no-op, got error: %v", err)
		}
	})

	t.Run("different target returns error", func(t *testing.T) {
		tmpDir := t.TempDir()
		ext1 := filepath.Join(tmpDir, "ext1")
		ext2 := filepath.Join(tmpDir, "ext2")
		os.MkdirAll(ext1, 0755)
		os.MkdirAll(ext2, 0755)
		servicePath := filepath.Join(tmpDir, "svc-diff")
		absExt1, _ := filepath.Abs(ext1)
		os.Symlink(absExt1, servicePath)

		err := CreateLink(servicePath, ext2)
		if err == nil {
			t.Error("expected error when symlink points to different target")
		}
	})
}

func TestRemoveLink(t *testing.T) {
	t.Run("remove existing symlink", func(t *testing.T) {
		tmpDir := t.TempDir()
		externalPath := filepath.Join(tmpDir, "external")
		os.MkdirAll(externalPath, 0755)
		linkPath := filepath.Join(tmpDir, "link")
		os.Symlink(externalPath, linkPath)

		err := RemoveLink(linkPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if _, err := os.Lstat(linkPath); !os.IsNotExist(err) {
			t.Error("expected symlink to be removed")
		}
	})

	t.Run("path does not exist", func(t *testing.T) {
		tmpDir := t.TempDir()
		err := RemoveLink(filepath.Join(tmpDir, "ghost"))
		if err == nil {
			t.Error("expected error for non-existent path")
		}
	})

	t.Run("path is not a symlink", func(t *testing.T) {
		tmpDir := t.TempDir()
		dirPath := filepath.Join(tmpDir, "real-dir")
		os.MkdirAll(dirPath, 0755)

		err := RemoveLink(dirPath)
		if err == nil {
			t.Error("expected error when path is not a symlink")
		}
	})
}

func TestGetServiceLinkPath(t *testing.T) {
	path, err := GetServiceLinkPath(nil, "service", nil)
	if err == nil {
		t.Error("expected error")
	}
	if path != "" {
		t.Error("expected empty path")
	}
}
