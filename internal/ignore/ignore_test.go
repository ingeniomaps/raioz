package ignore

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestGetIgnorePath(t *testing.T) {
	t.Run("returns valid path", func(t *testing.T) {
		// Use temp directory
		tmpDir := t.TempDir()
		os.Setenv("RAIOZ_HOME", tmpDir)
		defer os.Unsetenv("RAIOZ_HOME")

		path, err := GetIgnorePath()
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if path == "" {
			t.Error("Expected non-empty path")
		}
		if !filepath.IsAbs(path) {
			t.Error("Expected absolute path")
		}
		expected := filepath.Join(tmpDir, ignoreFileName)
		if path != expected {
			t.Errorf("Expected path %s, got %s", expected, path)
		}
	})
}

func TestLoad(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("RAIOZ_HOME", tmpDir)
	defer os.Unsetenv("RAIOZ_HOME")

	t.Run("load non-existent file returns empty config", func(t *testing.T) {
		config, err := Load()
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if config == nil {
			t.Fatal("Expected config, got nil")
		}
		if config.Services == nil {
			t.Error("Expected Services slice, got nil")
		}
		if len(config.Services) != 0 {
			t.Errorf("Expected empty Services, got %v", config.Services)
		}
	})

	t.Run("load existing file", func(t *testing.T) {
		// Create ignore file
		path, _ := GetIgnorePath()
		config := &IgnoreConfig{
			Services: []string{"service1", "service2"},
		}
		data, _ := json.Marshal(config)
		os.WriteFile(path, data, 0644)

		// Load it
		loaded, err := Load()
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if len(loaded.Services) != 2 {
			t.Errorf("Expected 2 services, got %d", len(loaded.Services))
		}
		if loaded.Services[0] != "service1" {
			t.Errorf("Expected service1, got %s", loaded.Services[0])
		}
	})

	t.Run("load corrupted file", func(t *testing.T) {
		path, _ := GetIgnorePath()
		os.WriteFile(path, []byte("invalid json"), 0644)

		_, err := Load()
		if err == nil {
			t.Error("Expected error loading corrupted ignore file, got nil")
		}

		// Clean up for next test
		os.Remove(path)
	})

	t.Run("load file with nil services", func(t *testing.T) {
		// Create ignore file with nil services
		path, _ := GetIgnorePath()
		data := []byte(`{}`)
		os.WriteFile(path, data, 0644)

		// Load it
		loaded, err := Load()
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if loaded.Services == nil {
			t.Error("Expected Services slice, got nil")
		}
		if len(loaded.Services) != 0 {
			t.Errorf("Expected empty Services, got %v", loaded.Services)
		}
	})
}

func TestSave(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("RAIOZ_HOME", tmpDir)
	defer os.Unsetenv("RAIOZ_HOME")

	t.Run("save config", func(t *testing.T) {
		config := &IgnoreConfig{
			Services: []string{"service1", "service2"},
		}

		err := Save(config)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		// Verify file exists
		path, _ := GetIgnorePath()
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Error("Ignore file should exist")
		}

		// Verify content
		data, _ := os.ReadFile(path)
		var loaded IgnoreConfig
		json.Unmarshal(data, &loaded)
		if len(loaded.Services) != 2 {
			t.Errorf("Expected 2 services, got %d", len(loaded.Services))
		}
	})

	t.Run("save creates directory if needed", func(t *testing.T) {
		// Use a nested path
		nestedDir := filepath.Join(tmpDir, "nested", "path")
		os.Setenv("RAIOZ_HOME", nestedDir)
		defer os.Unsetenv("RAIOZ_HOME")

		config := &IgnoreConfig{
			Services: []string{"service1"},
		}

		err := Save(config)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		// Verify directory was created
		if _, err := os.Stat(nestedDir); os.IsNotExist(err) {
			t.Error("Directory should be created")
		}
	})
}

func TestIsIgnored(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("RAIOZ_HOME", tmpDir)
	defer os.Unsetenv("RAIOZ_HOME")

	t.Run("service not ignored", func(t *testing.T) {
		ignored, err := IsIgnored("service1")
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if ignored {
			t.Error("Expected service not to be ignored")
		}
	})

	t.Run("service is ignored", func(t *testing.T) {
		// Add service to ignore list
		err := AddService("service1")
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		ignored, err := IsIgnored("service1")
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if !ignored {
			t.Error("Expected service to be ignored")
		}
	})
}

func TestAddService(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("RAIOZ_HOME", tmpDir)
	defer os.Unsetenv("RAIOZ_HOME")

	t.Run("add new service", func(t *testing.T) {
		err := AddService("service1")
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		// Verify it was added
		ignored, _ := IsIgnored("service1")
		if !ignored {
			t.Error("Expected service to be ignored")
		}
	})

	t.Run("add duplicate service is no-op", func(t *testing.T) {
		// Add service twice
		AddService("service2")
		err := AddService("service2")
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		// Verify it's still in the list (only once)
		services, _ := GetIgnoredServices()
		count := 0
		for _, s := range services {
			if s == "service2" {
				count++
			}
		}
		if count != 1 {
			t.Errorf("Expected service2 to appear once, found %d times", count)
		}
	})
}

func TestRemoveService(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("RAIOZ_HOME", tmpDir)
	defer os.Unsetenv("RAIOZ_HOME")

	t.Run("remove existing service", func(t *testing.T) {
		// Add service first
		AddService("service1")

		// Remove it
		err := RemoveService("service1")
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		// Verify it was removed
		ignored, _ := IsIgnored("service1")
		if ignored {
			t.Error("Expected service not to be ignored")
		}
	})

	t.Run("remove non-existent service is no-op", func(t *testing.T) {
		err := RemoveService("nonexistent")
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
	})
}

func TestGetIgnoredServices(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("RAIOZ_HOME", tmpDir)
	defer os.Unsetenv("RAIOZ_HOME")

	t.Run("empty list", func(t *testing.T) {
		services, err := GetIgnoredServices()
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if len(services) != 0 {
			t.Errorf("Expected empty list, got %v", services)
		}
	})

	t.Run("list with services", func(t *testing.T) {
		// Add services
		AddService("service1")
		AddService("service2")

		services, err := GetIgnoredServices()
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if len(services) != 2 {
			t.Errorf("Expected 2 services, got %d", len(services))
		}
	})
}
