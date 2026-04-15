package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"raioz/internal/config"
	"raioz/internal/mocks"
)

func TestFindConfigFile_None(t *testing.T) {
	// Switch cwd to empty dir where no config files exist
	orig, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(orig) })
	_ = os.Chdir(t.TempDir())

	if got := findConfigFile(); got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestFindConfigFile_RaiozYaml(t *testing.T) {
	orig, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(orig) })
	tmpDir := t.TempDir()
	_ = os.Chdir(tmpDir)

	if err := os.WriteFile("raioz.yaml", []byte("project: test\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if got := findConfigFile(); got != "raioz.yaml" {
		t.Errorf("expected 'raioz.yaml', got %q", got)
	}
}

func TestFindConfigFile_LegacyJSON(t *testing.T) {
	orig, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(orig) })
	tmpDir := t.TempDir()
	_ = os.Chdir(tmpDir)

	if err := os.WriteFile(".raioz.json", []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	if got := findConfigFile(); got != ".raioz.json" {
		t.Errorf("expected '.raioz.json', got %q", got)
	}
}

func TestResolveYAMLProject_Nil(t *testing.T) {
	deps := newFullMockDeps()
	deps.ConfigLoader = &mocks.MockConfigLoader{
		LoadDepsFunc: func(configPath string) (*config.Deps, []string, error) {
			return nil, nil, nil
		},
	}
	if proj := ResolveYAMLProject(deps, "does-not-exist.yaml"); proj != nil {
		t.Errorf("expected nil, got %+v", proj)
	}
}

func TestResolveYAMLProject_Legacy(t *testing.T) {
	deps := newFullMockDeps()
	deps.ConfigLoader = &mocks.MockConfigLoader{
		LoadDepsFunc: func(configPath string) (*config.Deps, []string, error) {
			return &config.Deps{
				Project:       config.Project{Name: "test"},
				SchemaVersion: "1.0",
			}, nil, nil
		},
	}
	if proj := ResolveYAMLProject(deps, "legacy.json"); proj != nil {
		t.Errorf("expected nil for legacy schema, got %+v", proj)
	}
}

func TestResolveYAMLProject_YAMLMode(t *testing.T) {
	deps := newFullMockDeps()
	deps.ConfigLoader = &mocks.MockConfigLoader{
		LoadDepsFunc: func(configPath string) (*config.Deps, []string, error) {
			return &config.Deps{
				Project:       config.Project{Name: "yamlproj"},
				Network:       config.NetworkConfig{Name: "yamlproj-net"},
				SchemaVersion: "2.0",
			}, nil, nil
		},
	}
	proj := ResolveYAMLProject(deps, "raioz.yaml")
	if proj == nil {
		t.Fatal("expected YAMLProject, got nil")
	}
	if proj.ProjectName != "yamlproj" {
		t.Errorf("expected project name 'yamlproj', got %q", proj.ProjectName)
	}
	if !filepath.IsAbs(proj.ConfigPath) && proj.ConfigPath != "raioz.yaml" {
		t.Errorf("unexpected config path: %q", proj.ConfigPath)
	}
}

func TestResolveYAMLProject_AutoFindEmpty(t *testing.T) {
	orig, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(orig) })
	_ = os.Chdir(t.TempDir())

	deps := newFullMockDeps()
	if proj := ResolveYAMLProject(deps, ""); proj != nil {
		t.Errorf("expected nil with no config file, got %+v", proj)
	}
}

func TestYAMLProject_ContainerPrefix(t *testing.T) {
	p := &YAMLProject{ProjectName: "myapp"}
	if got := p.ContainerPrefix(); got != "raioz-myapp-" {
		t.Errorf("expected 'raioz-myapp-', got %q", got)
	}
}

func TestYAMLProject_ContainerStatus_Stopped(t *testing.T) {
	// No container will exist — expect "stopped"
	p := &YAMLProject{ProjectName: "zzz-test-nonexistent-proj"}
	status := p.ContainerStatus(context.Background(), "noservice")
	if status != "stopped" {
		t.Errorf("expected 'stopped' for nonexistent container, got %q", status)
	}
}

func TestYAMLProject_ContainerStats_Stopped(t *testing.T) {
	p := &YAMLProject{ProjectName: "zzz-test-nonexistent-proj"}
	cpu, mem := p.ContainerStats(context.Background(), "noservice")
	if cpu != "-" || mem != "-" {
		t.Errorf("expected ('-', '-') for nonexistent container, got (%q, %q)", cpu, mem)
	}
}

func TestYAMLProject_ListRunningContainers_Empty(t *testing.T) {
	p := &YAMLProject{ProjectName: "zzz-test-nonexistent-proj-list"}
	names := p.ListRunningContainers(context.Background())
	if len(names) != 0 {
		t.Errorf("expected empty list, got %v", names)
	}
}
