package override

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestGetOverridesPath(t *testing.T) {
	t.Run("returns valid path", func(t *testing.T) {
		tmpDir := t.TempDir()
		os.Setenv("RAIOZ_HOME", tmpDir)
		defer os.Unsetenv("RAIOZ_HOME")

		path, err := GetOverridesPath()
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if path == "" {
			t.Error("Expected non-empty path")
		}
		expected := filepath.Join(tmpDir, overridesFileName)
		if path != expected {
			t.Errorf("Expected path %s, got %s", expected, path)
		}
	})
}

func TestLoadOverrides(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("RAIOZ_HOME", tmpDir)
	defer os.Unsetenv("RAIOZ_HOME")

	t.Run("load non-existent file returns empty map", func(t *testing.T) {
		overrides, err := LoadOverrides()
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if overrides == nil {
			t.Fatal("Expected map, got nil")
		}
		if len(overrides) != 0 {
			t.Errorf("Expected empty map, got %v", overrides)
		}
	})

	t.Run("load existing file", func(t *testing.T) {
		// Create overrides file
		path, _ := GetOverridesPath()
		overrides := Overrides{
			"service1": Override{
				Path:   "/path/to/service1",
				Mode:   "local",
				Source: "external",
			},
		}
		data, _ := json.Marshal(overrides)
		os.WriteFile(path, data, 0644)

		// Load it
		loaded, err := LoadOverrides()
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if len(loaded) != 1 {
			t.Errorf("Expected 1 override, got %d", len(loaded))
		}
		if loaded["service1"].Path != "/path/to/service1" {
			t.Errorf("Expected path /path/to/service1, got %s", loaded["service1"].Path)
		}
	})

	t.Run("load corrupted file", func(t *testing.T) {
		path, _ := GetOverridesPath()
		os.WriteFile(path, []byte("invalid json"), 0644)

		_, err := LoadOverrides()
		if err == nil {
			t.Error("Expected error loading corrupted override file, got nil")
		}

		// Clean up for next test
		os.Remove(path)
	})

	t.Run("load file with nil map", func(t *testing.T) {
		// Create overrides file with null
		path, _ := GetOverridesPath()
		data := []byte(`null`)
		os.WriteFile(path, data, 0644)

		// Load it
		loaded, err := LoadOverrides()
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if loaded == nil {
			t.Error("Expected map, got nil")
		}
		if len(loaded) != 0 {
			t.Errorf("Expected empty map, got %v", loaded)
		}
	})
}

func TestSaveOverrides(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("RAIOZ_HOME", tmpDir)
	defer os.Unsetenv("RAIOZ_HOME")

	t.Run("save overrides", func(t *testing.T) {
		overrides := Overrides{
			"service1": Override{
				Path:   "/path/to/service1",
				Mode:   "local",
				Source: "external",
			},
		}

		err := SaveOverrides(overrides)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		// Verify file exists
		path, _ := GetOverridesPath()
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Error("Overrides file should exist")
		}

		// Verify content
		data, _ := os.ReadFile(path)
		var loaded Overrides
		json.Unmarshal(data, &loaded)
		if len(loaded) != 1 {
			t.Errorf("Expected 1 override, got %d", len(loaded))
		}
	})
}

func TestAddOverride(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("RAIOZ_HOME", tmpDir)
	defer os.Unsetenv("RAIOZ_HOME")

	t.Run("add new override", func(t *testing.T) {
		override := Override{
			Path:   "/path/to/service1",
			Mode:   "local",
			Source: "external",
		}

		err := AddOverride("service1", override)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		// Verify it was added
		loaded, _ := LoadOverrides()
		if len(loaded) != 1 {
			t.Errorf("Expected 1 override, got %d", len(loaded))
		}
		if loaded["service1"].Path != "/path/to/service1" {
			t.Errorf("Expected path /path/to/service1, got %s", loaded["service1"].Path)
		}
	})

	t.Run("update existing override", func(t *testing.T) {
		// Add first override
		AddOverride("service1", Override{Path: "/old/path"})

		// Update it
		newOverride := Override{
			Path:   "/new/path",
			Mode:   "local",
			Source: "external",
		}
		err := AddOverride("service1", newOverride)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		// Verify it was updated
		loaded, _ := LoadOverrides()
		if loaded["service1"].Path != "/new/path" {
			t.Errorf("Expected path /new/path, got %s", loaded["service1"].Path)
		}
	})
}

func TestRemoveOverride(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("RAIOZ_HOME", tmpDir)
	defer os.Unsetenv("RAIOZ_HOME")

	t.Run("remove existing override", func(t *testing.T) {
		// Add override first
		AddOverride("service1", Override{Path: "/path/to/service1"})

		// Remove it
		err := RemoveOverride("service1")
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		// Verify it was removed
		loaded, _ := LoadOverrides()
		if len(loaded) != 0 {
			t.Errorf("Expected empty map, got %v", loaded)
		}
	})

	t.Run("remove non-existent override", func(t *testing.T) {
		err := RemoveOverride("nonexistent")
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
	})
}

func TestGetOverride(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("RAIOZ_HOME", tmpDir)
	defer os.Unsetenv("RAIOZ_HOME")

	t.Run("get existing override", func(t *testing.T) {
		// Add override
		AddOverride("service1", Override{Path: "/path/to/service1"})

		override, err := GetOverride("service1")
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if override == nil {
			t.Fatal("Expected override, got nil")
		}
		if override.Path != "/path/to/service1" {
			t.Errorf("Expected path /path/to/service1, got %s", override.Path)
		}
	})

	t.Run("get non-existent override", func(t *testing.T) {
		override, err := GetOverride("nonexistent")
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if override != nil {
			t.Error("Expected nil override, got non-nil")
		}
	})
}

func TestHasOverride(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("RAIOZ_HOME", tmpDir)
	defer os.Unsetenv("RAIOZ_HOME")

	t.Run("has override", func(t *testing.T) {
		// Add override
		AddOverride("service1", Override{Path: "/path/to/service1"})

		has, err := HasOverride("service1")
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if !has {
			t.Error("Expected override to exist")
		}
	})

	t.Run("no override", func(t *testing.T) {
		has, err := HasOverride("nonexistent")
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if has {
			t.Error("Expected no override")
		}
	})
}

func TestValidateOverridePath(t *testing.T) {
	t.Run("valid directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		err := ValidateOverridePath(tmpDir)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
	})

	t.Run("path does not exist", func(t *testing.T) {
		err := ValidateOverridePath("/nonexistent/path")
		if err == nil {
			t.Error("Expected error for non-existent path")
		}
	})

	t.Run("path is not a directory", func(t *testing.T) {
		tmpFile := filepath.Join(t.TempDir(), "file.txt")
		os.WriteFile(tmpFile, []byte("test"), 0644)

		err := ValidateOverridePath(tmpFile)
		if err == nil {
			t.Error("Expected error for file (not directory)")
		}
	})
}

func TestCleanInvalidOverrides(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("RAIOZ_HOME", tmpDir)
	defer os.Unsetenv("RAIOZ_HOME")

	t.Run("clean invalid overrides", func(t *testing.T) {
		// Add valid override
		validPath := t.TempDir()
		AddOverride("valid", Override{Path: validPath})

		// Add invalid override
		AddOverride("invalid", Override{Path: "/nonexistent/path"})

		// Clean invalid overrides
		removed, err := CleanInvalidOverrides()
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if len(removed) != 1 {
			t.Errorf("Expected 1 removed override, got %d", len(removed))
		}
		if removed[0] != "invalid" {
			t.Errorf("Expected 'invalid' to be removed, got %s", removed[0])
		}

		// Verify valid override still exists
		has, _ := HasOverride("valid")
		if !has {
			t.Error("Expected valid override to still exist")
		}

		// Verify invalid override was removed
		has, _ = HasOverride("invalid")
		if has {
			t.Error("Expected invalid override to be removed")
		}
	})

	t.Run("no invalid overrides", func(t *testing.T) {
		// Add only valid override
		validPath := t.TempDir()
		AddOverride("valid", Override{Path: validPath})

		removed, err := CleanInvalidOverrides()
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if len(removed) != 0 {
			t.Errorf("Expected no removed overrides, got %d", len(removed))
		}
	})
}
