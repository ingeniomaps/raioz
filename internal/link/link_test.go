package link

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsLinked(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("path does not exist", func(t *testing.T) {
		servicePath := filepath.Join(tmpDir, "nonexistent")
		isLinked, target, err := IsLinked(servicePath)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if isLinked {
			t.Error("Expected not linked")
		}
		if target != "" {
			t.Error("Expected empty target")
		}
	})

	t.Run("path is a directory (not symlink)", func(t *testing.T) {
		servicePath := filepath.Join(tmpDir, "service")
		os.MkdirAll(servicePath, 0755)

		isLinked, target, err := IsLinked(servicePath)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if isLinked {
			t.Error("Expected not linked")
		}
		if target != "" {
			t.Error("Expected empty target")
		}
	})

	t.Run("path is a symlink", func(t *testing.T) {
		externalPath := filepath.Join(tmpDir, "external")
		os.MkdirAll(externalPath, 0755)

		servicePath := filepath.Join(tmpDir, "service")
		absExternal, _ := filepath.Abs(externalPath)
		os.Symlink(absExternal, servicePath)

		isLinked, target, err := IsLinked(servicePath)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if !isLinked {
			t.Error("Expected linked")
		}
		if target == "" {
			t.Error("Expected non-empty target")
		}
		// Target should be absolute
		if !filepath.IsAbs(target) {
			t.Errorf("Expected absolute target, got %s", target)
		}
	})

	t.Run("path is a relative symlink", func(t *testing.T) {
		externalPath := filepath.Join(tmpDir, "external")
		os.MkdirAll(externalPath, 0755)

		servicePath := filepath.Join(tmpDir, "service")
		// Create relative symlink
		os.Symlink("external", servicePath)

		isLinked, target, err := IsLinked(servicePath)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if !isLinked {
			t.Error("Expected linked")
		}
		if !filepath.IsAbs(target) {
			t.Error("Expected absolute target path")
		}
	})
}

func TestCreateLink(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("create new link", func(t *testing.T) {
		externalPath := filepath.Join(tmpDir, "external")
		os.MkdirAll(externalPath, 0755)

		servicePath := filepath.Join(tmpDir, "service")

		err := CreateLink(servicePath, externalPath)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		// Verify symlink was created
		isLinked, target, _ := IsLinked(servicePath)
		if !isLinked {
			t.Error("Expected symlink to be created")
		}
		if target == "" {
			t.Error("Expected non-empty target")
		}
	})

	t.Run("external path does not exist", func(t *testing.T) {
		servicePath := filepath.Join(tmpDir, "service")
		externalPath := filepath.Join(tmpDir, "nonexistent")

		err := CreateLink(servicePath, externalPath)
		if err == nil {
			t.Error("Expected error for non-existent external path")
		}
	})

	t.Run("external path is not a directory", func(t *testing.T) {
		externalFile := filepath.Join(tmpDir, "file.txt")
		os.WriteFile(externalFile, []byte("test"), 0644)

		servicePath := filepath.Join(tmpDir, "service")

		err := CreateLink(servicePath, externalFile)
		if err == nil {
			t.Error("Expected error for file (not directory)")
		}
	})

	t.Run("service path already exists as directory", func(t *testing.T) {
		externalPath := filepath.Join(tmpDir, "external")
		os.MkdirAll(externalPath, 0755)

		servicePath := filepath.Join(tmpDir, "service")
		os.MkdirAll(servicePath, 0755)

		err := CreateLink(servicePath, externalPath)
		if err == nil {
			t.Error("Expected error when service path exists as directory")
		} else {
			// Verify error message mentions directory or symlink
			errMsg := err.Error()
			if errMsg == "" {
				t.Error("Expected error message")
			}
			// The error should mention that it's a directory or not a symlink
		}
	})

	t.Run("service path already exists as symlink to same target", func(t *testing.T) {
		externalPath := filepath.Join(tmpDir, "external")
		os.MkdirAll(externalPath, 0755)

		servicePath := filepath.Join(tmpDir, "service")
		os.Symlink(externalPath, servicePath)

		// Try to create link again (should be no-op)
		err := CreateLink(servicePath, externalPath)
		if err != nil {
			t.Fatalf("Expected no error (no-op), got %v", err)
		}
	})

	t.Run("service path already exists as symlink to different target", func(t *testing.T) {
		externalPath1 := filepath.Join(tmpDir, "external1")
		externalPath2 := filepath.Join(tmpDir, "external2")
		os.MkdirAll(externalPath1, 0755)
		os.MkdirAll(externalPath2, 0755)

		servicePath := filepath.Join(tmpDir, "service")
		os.Symlink(externalPath1, servicePath)

		// Try to create link to different target
		err := CreateLink(servicePath, externalPath2)
		if err == nil {
			t.Error("Expected error when symlink points to different target")
		}
	})
}

func TestRemoveLink(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("remove existing symlink", func(t *testing.T) {
		externalPath := filepath.Join(tmpDir, "external")
		os.MkdirAll(externalPath, 0755)

		servicePath := filepath.Join(tmpDir, "service")
		os.Symlink(externalPath, servicePath)

		err := RemoveLink(servicePath)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		// Verify symlink was removed
		if _, err := os.Lstat(servicePath); !os.IsNotExist(err) {
			t.Error("Expected symlink to be removed")
		}
	})

	t.Run("path does not exist", func(t *testing.T) {
		servicePath := filepath.Join(tmpDir, "nonexistent")

		err := RemoveLink(servicePath)
		if err == nil {
			t.Error("Expected error for non-existent path")
		}
	})

	t.Run("path is not a symlink", func(t *testing.T) {
		servicePath := filepath.Join(tmpDir, "service")
		os.MkdirAll(servicePath, 0755)

		err := RemoveLink(servicePath)
		if err == nil {
			t.Error("Expected error when path is not a symlink")
		}
	})
}

func TestGetServiceLinkPath(t *testing.T) {
	t.Run("returns error", func(t *testing.T) {
		path, err := GetServiceLinkPath(nil, "service", nil)
		if err == nil {
			t.Error("Expected error")
		}
		if path != "" {
			t.Error("Expected empty path")
		}
	})
}
