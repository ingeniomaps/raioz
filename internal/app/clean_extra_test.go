package app

import (
	"context"
	"testing"

	"raioz/internal/config"
)

func TestCleanUseCase_Execute_CleanVolumesWithForce(t *testing.T) {
	initI18nForTest(t)
	deps, configLoader, _, dockerRunner := newTestDepsForClean(t)
	configLoader.LoadDepsFunc = func(configPath string) (*config.Deps, []string, error) {
		return &config.Deps{Project: config.Project{Name: "test-project"}}, nil, nil
	}
	var cleanVolumesCalled bool
	dockerRunner.CleanUnusedVolumesWithContextFunc = func(ctx context.Context, dryRun, force bool) ([]string, error) {
		cleanVolumesCalled = true
		return []string{"Removed volume foo"}, nil
	}
	dockerRunner.CleanProjectWithContextFunc = func(ctx context.Context, composePath string, dryRun bool) ([]string, error) {
		return nil, nil
	}
	uc := NewCleanUseCase(deps)
	err := uc.Execute(context.Background(), CleanOptions{
		ProjectName: "test-project",
		Volumes:     true,
		Force:       true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cleanVolumesCalled {
		t.Error("expected CleanUnusedVolumesWithContext to be called")
	}
}

func TestCleanUseCase_Execute_CleanVolumesDryRun(t *testing.T) {
	initI18nForTest(t)
	deps, configLoader, _, dockerRunner := newTestDepsForClean(t)
	configLoader.LoadDepsFunc = func(configPath string) (*config.Deps, []string, error) {
		return &config.Deps{Project: config.Project{Name: "test-project"}}, nil, nil
	}
	dockerRunner.CleanUnusedVolumesWithContextFunc = func(ctx context.Context, dryRun, force bool) ([]string, error) {
		if !dryRun {
			t.Error("expected dryRun=true")
		}
		return nil, nil
	}
	dockerRunner.CleanProjectWithContextFunc = func(ctx context.Context, composePath string, dryRun bool) ([]string, error) {
		return nil, nil
	}
	uc := NewCleanUseCase(deps)
	err := uc.Execute(context.Background(), CleanOptions{
		ProjectName: "test-project",
		Volumes:     true,
		DryRun:      true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCleanUseCase_Execute_Networks(t *testing.T) {
	initI18nForTest(t)
	deps, configLoader, _, dockerRunner := newTestDepsForClean(t)
	configLoader.LoadDepsFunc = func(configPath string) (*config.Deps, []string, error) {
		return &config.Deps{Project: config.Project{Name: "test-project"}}, nil, nil
	}
	var networksCalled bool
	dockerRunner.CleanUnusedNetworksWithContextFunc = func(ctx context.Context, dryRun bool) ([]string, error) {
		networksCalled = true
		return []string{"Removed network foo"}, nil
	}
	dockerRunner.CleanProjectWithContextFunc = func(ctx context.Context, composePath string, dryRun bool) ([]string, error) {
		return nil, nil
	}
	uc := NewCleanUseCase(deps)
	err := uc.Execute(context.Background(), CleanOptions{
		ProjectName: "test-project",
		Networks:    true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !networksCalled {
		t.Error("expected CleanUnusedNetworksWithContext to be called")
	}
}

func TestCleanUseCase_Execute_Images(t *testing.T) {
	initI18nForTest(t)
	deps, configLoader, _, dockerRunner := newTestDepsForClean(t)
	configLoader.LoadDepsFunc = func(configPath string) (*config.Deps, []string, error) {
		return &config.Deps{Project: config.Project{Name: "test-project"}}, nil, nil
	}
	var imagesCalled bool
	dockerRunner.CleanUnusedImagesWithContextFunc = func(ctx context.Context, dryRun bool) ([]string, error) {
		imagesCalled = true
		return nil, nil
	}
	dockerRunner.CleanProjectWithContextFunc = func(ctx context.Context, composePath string, dryRun bool) ([]string, error) {
		return nil, nil
	}
	uc := NewCleanUseCase(deps)
	err := uc.Execute(context.Background(), CleanOptions{
		ProjectName: "test-project",
		Images:      true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !imagesCalled {
		t.Error("expected CleanUnusedImagesWithContext to be called")
	}
}
