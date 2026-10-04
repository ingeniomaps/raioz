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
		services, _ := ForProject("")
		if !contains(services, "service1") {
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
		services, _ := ForProject("")
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

// Ignoring a service in one project says nothing about another project's
// service of the same name; the pre-existing global list still applies.
func TestIgnore_PerProject(t *testing.T) {
	t.Setenv("RAIOZ_HOME", t.TempDir())

	if err := AddService("legacy"); err != nil {
		t.Fatal(err)
	}
	if err := AddFor("alpha", "api"); err != nil {
		t.Fatal(err)
	}

	alpha, _ := ForProject("alpha")
	beta, _ := ForProject("beta")
	if !contains(alpha, "api") || !contains(alpha, "legacy") {
		t.Errorf("alpha = %v, want api and the legacy entry", alpha)
	}
	if contains(beta, "api") {
		t.Errorf("beta = %v: alpha's ignore leaked into another project", beta)
	}
	if !contains(beta, "legacy") {
		t.Errorf("beta = %v: the legacy global entry must still apply", beta)
	}

	if err := RemoveFor("alpha", "api"); err != nil {
		t.Fatal(err)
	}
	if err := RemoveFor("alpha", "legacy"); err != nil {
		t.Fatal(err)
	}
	if alpha, _ = ForProject("alpha"); len(alpha) != 0 {
		t.Errorf("alpha after removal = %v, want empty", alpha)
	}
}
