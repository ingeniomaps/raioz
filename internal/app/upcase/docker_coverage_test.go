package upcase

import (
	"context"
	stderrors "errors"
	"testing"

	"raioz/internal/config"
	"raioz/internal/domain/interfaces"
	"raioz/internal/mocks"
	"raioz/internal/workspace"
)

// --- prepareDockerResources: volume extraction and creation ------------------

func TestPrepareDockerResourcesWithVolumes(t *testing.T) {
	initI18nUp(t)

	volumesCreated := 0
	uc := NewUseCase(&Dependencies{
		Workspace: &mocks.MockWorkspaceManager{
			GetBaseDirFromWorkspaceFunc: func(ws *workspace.Workspace) string { return "/base" },
		},
		DockerRunner: &mocks.MockDockerRunner{
			ValidatePortsFunc: func(d *config.Deps, b string, p string) ([]interfaces.PortConflict, error) {
				return nil, nil
			},
			ValidateAllImagesFunc: func(*config.Deps) error { return nil },
			EnsureNetworkWithConfigAndContextFunc: func(ctx context.Context, name, subnet string, ask bool) error {
				return nil
			},
			ExtractNamedVolumesFunc: func(v []string) ([]string, error) {
				if len(v) > 0 {
					// Extract volume name from "name:/path" format
					for _, vol := range v {
						for i := 0; i < len(vol); i++ {
							if vol[i] == ':' {
								return []string{vol[:i]}, nil
							}
						}
					}
					return []string{"data"}, nil
				}
				return nil, nil
			},
			NormalizeVolumeNameFunc: func(p, n string) (string, error) {
				return p + "-" + n, nil
			},
			EnsureVolumeWithContextFunc: func(ctx context.Context, name string) error {
				volumesCreated++
				return nil
			},
		},
	})

	deps := &config.Deps{
		SchemaVersion: "2.0",
		Project:       config.Project{Name: "p"},
		Network:       config.NetworkConfig{Name: "net"},
		Infra: map[string]config.InfraEntry{
			"db": {Inline: &config.Infra{Image: "postgres", Volumes: []string{"pgdata:/var/lib/postgresql"}}},
		},
		Services: map[string]config.Service{
			"api": {Docker: &config.DockerConfig{Volumes: []string{"uploads:/uploads"}}},
		},
	}

	err := uc.prepareDockerResources(context.Background(), deps, &workspace.Workspace{Root: "/t"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if volumesCreated != 2 {
		t.Errorf("expected 2 volumes created, got %d", volumesCreated)
	}
}

func TestPrepareDockerResourcesVolumeExtractError(t *testing.T) {
	initI18nUp(t)

	uc := NewUseCase(&Dependencies{
		Workspace: &mocks.MockWorkspaceManager{
			GetBaseDirFromWorkspaceFunc: func(ws *workspace.Workspace) string { return "/base" },
		},
		DockerRunner: &mocks.MockDockerRunner{
			ValidatePortsFunc: func(d *config.Deps, b string, p string) ([]interfaces.PortConflict, error) {
				return nil, nil
			},
			ValidateAllImagesFunc: func(*config.Deps) error { return nil },
			EnsureNetworkWithConfigAndContextFunc: func(ctx context.Context, name, subnet string, ask bool) error {
				return nil
			},
			ExtractNamedVolumesFunc: func(v []string) ([]string, error) {
				return nil, stderrors.New("extract error")
			},
		},
	})

	deps := &config.Deps{
		SchemaVersion: "2.0",
		Project:       config.Project{Name: "p"},
		Network:       config.NetworkConfig{Name: "net"},
		Infra: map[string]config.InfraEntry{
			"db": {Inline: &config.Infra{Image: "postgres", Volumes: []string{"data:/var"}}},
		},
	}

	err := uc.prepareDockerResources(context.Background(), deps, &workspace.Workspace{Root: "/t"})
	if err == nil {
		t.Error("expected volume extract error")
	}
}

func TestPrepareDockerResourcesVolumeNormalizeError(t *testing.T) {
	initI18nUp(t)

	uc := NewUseCase(&Dependencies{
		Workspace: &mocks.MockWorkspaceManager{
			GetBaseDirFromWorkspaceFunc: func(ws *workspace.Workspace) string { return "/base" },
		},
		DockerRunner: &mocks.MockDockerRunner{
			ValidatePortsFunc: func(d *config.Deps, b string, p string) ([]interfaces.PortConflict, error) {
				return nil, nil
			},
			ValidateAllImagesFunc: func(*config.Deps) error { return nil },
			EnsureNetworkWithConfigAndContextFunc: func(ctx context.Context, name, subnet string, ask bool) error {
				return nil
			},
			ExtractNamedVolumesFunc: func(v []string) ([]string, error) {
				return []string{"data"}, nil
			},
			NormalizeVolumeNameFunc: func(p, n string) (string, error) {
				return "", stderrors.New("normalize error")
			},
		},
	})

	deps := &config.Deps{
		SchemaVersion: "2.0",
		Project:       config.Project{Name: "p"},
		Network:       config.NetworkConfig{Name: "net"},
		Infra: map[string]config.InfraEntry{
			"db": {Inline: &config.Infra{Image: "postgres", Volumes: []string{"data:/var"}}},
		},
	}

	err := uc.prepareDockerResources(context.Background(), deps, &workspace.Workspace{Root: "/t"})
	if err == nil {
		t.Error("expected volume normalize error")
	}
}

func TestPrepareDockerResourcesVolumeEnsureError(t *testing.T) {
	initI18nUp(t)

	uc := NewUseCase(&Dependencies{
		Workspace: &mocks.MockWorkspaceManager{
			GetBaseDirFromWorkspaceFunc: func(ws *workspace.Workspace) string { return "/base" },
		},
		DockerRunner: &mocks.MockDockerRunner{
			ValidatePortsFunc: func(d *config.Deps, b string, p string) ([]interfaces.PortConflict, error) {
				return nil, nil
			},
			ValidateAllImagesFunc: func(*config.Deps) error { return nil },
			EnsureNetworkWithConfigAndContextFunc: func(ctx context.Context, name, subnet string, ask bool) error {
				return nil
			},
			ExtractNamedVolumesFunc: func(v []string) ([]string, error) {
				return []string{"data"}, nil
			},
			NormalizeVolumeNameFunc: func(p, n string) (string, error) {
				return p + "-" + n, nil
			},
			EnsureVolumeWithContextFunc: func(ctx context.Context, name string) error {
				return stderrors.New("ensure volume failed")
			},
		},
	})

	deps := &config.Deps{
		SchemaVersion: "2.0",
		Project:       config.Project{Name: "p"},
		Network:       config.NetworkConfig{Name: "net"},
		Infra: map[string]config.InfraEntry{
			"db": {Inline: &config.Infra{Image: "postgres", Volumes: []string{"data:/var"}}},
		},
	}

	err := uc.prepareDockerResources(context.Background(), deps, &workspace.Workspace{Root: "/t"})
	if err == nil {
		t.Error("expected volume ensure error")
	}
}

// --- prepareDockerResources: service volume extract error ---------------------

func TestPrepareDockerResourcesServiceVolumeExtractError(t *testing.T) {
	initI18nUp(t)

	callCount := 0
	uc := NewUseCase(&Dependencies{
		Workspace: &mocks.MockWorkspaceManager{
			GetBaseDirFromWorkspaceFunc: func(ws *workspace.Workspace) string { return "/base" },
		},
		DockerRunner: &mocks.MockDockerRunner{
			ValidatePortsFunc: func(d *config.Deps, b string, p string) ([]interfaces.PortConflict, error) {
				return nil, nil
			},
			ValidateAllImagesFunc: func(*config.Deps) error { return nil },
			EnsureNetworkWithConfigAndContextFunc: func(ctx context.Context, name, subnet string, ask bool) error {
				return nil
			},
			ExtractNamedVolumesFunc: func(v []string) ([]string, error) {
				callCount++
				if callCount > 0 {
					return nil, stderrors.New("service volume extract error")
				}
				return nil, nil
			},
		},
	})

	deps := &config.Deps{
		SchemaVersion: "2.0",
		Project:       config.Project{Name: "p"},
		Network:       config.NetworkConfig{Name: "net"},
		Services: map[string]config.Service{
			"api": {Docker: &config.DockerConfig{Volumes: []string{"data:/data"}}},
		},
	}

	err := uc.prepareDockerResources(context.Background(), deps, &workspace.Workspace{Root: "/t"})
	if err == nil {
		t.Error("expected service volume extract error")
	}
}

// --- prepareDockerResources: legacy schema (askConfirmation) ------------------

func TestPrepareDockerResourcesLegacySchema(t *testing.T) {
	initI18nUp(t)

	var gotAsk bool
	uc := NewUseCase(&Dependencies{
		Workspace: &mocks.MockWorkspaceManager{
			GetBaseDirFromWorkspaceFunc: func(ws *workspace.Workspace) string { return "/base" },
		},
		DockerRunner: &mocks.MockDockerRunner{
			ValidatePortsFunc: func(d *config.Deps, b string, p string) ([]interfaces.PortConflict, error) {
				return nil, nil
			},
			ValidateAllImagesFunc: func(*config.Deps) error { return nil },
			EnsureNetworkWithConfigAndContextFunc: func(ctx context.Context, name, subnet string, ask bool) error {
				gotAsk = ask
				return nil
			},
			ExtractNamedVolumesFunc: func(v []string) ([]string, error) { return nil, nil },
		},
	})

	deps := &config.Deps{
		SchemaVersion: "1.0", // Legacy
		Project:       config.Project{Name: "p"},
		Network:       config.NetworkConfig{Name: "net"},
	}

	err := uc.prepareDockerResources(context.Background(), deps, &workspace.Workspace{Root: "/t"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !gotAsk {
		t.Error("legacy schema should ask for confirmation")
	}
}

func TestPrepareDockerResourcesYAMLSchemaNoAsk(t *testing.T) {
	initI18nUp(t)

	var gotAsk bool
	uc := NewUseCase(&Dependencies{
		Workspace: &mocks.MockWorkspaceManager{
			GetBaseDirFromWorkspaceFunc: func(ws *workspace.Workspace) string { return "/base" },
		},
		DockerRunner: &mocks.MockDockerRunner{
			ValidatePortsFunc: func(d *config.Deps, b string, p string) ([]interfaces.PortConflict, error) {
				return nil, nil
			},
			ValidateAllImagesFunc: func(*config.Deps) error { return nil },
			EnsureNetworkWithConfigAndContextFunc: func(ctx context.Context, name, subnet string, ask bool) error {
				gotAsk = ask
				return nil
			},
			ExtractNamedVolumesFunc: func(v []string) ([]string, error) { return nil, nil },
		},
	})

	deps := &config.Deps{
		SchemaVersion: "2.0", // YAML
		Project:       config.Project{Name: "p"},
		Network:       config.NetworkConfig{Name: "net"},
	}

	err := uc.prepareDockerResources(context.Background(), deps, &workspace.Workspace{Root: "/t"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotAsk {
		t.Error("YAML schema (2.0) should NOT ask for confirmation")
	}
}
