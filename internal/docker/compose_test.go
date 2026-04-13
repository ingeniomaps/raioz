package docker

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"raioz/internal/config"
	"raioz/internal/workspace"
)

// mkWorkspace builds a workspace rooted at tmp with all subdirs created.
func mkWorkspace(tmp string) *workspace.Workspace {
	ws := &workspace.Workspace{
		Root:                tmp,
		ServicesDir:         filepath.Join(tmp, "services"),
		LocalServicesDir:    filepath.Join(tmp, "services", "local"),
		ReadonlyServicesDir: filepath.Join(tmp, "services", "readonly"),
		EnvDir:              filepath.Join(tmp, "env"),
	}
	_ = os.MkdirAll(ws.Root, 0755)
	_ = os.MkdirAll(ws.ServicesDir, 0755)
	_ = os.MkdirAll(ws.LocalServicesDir, 0755)
	_ = os.MkdirAll(ws.ReadonlyServicesDir, 0755)
	_ = os.MkdirAll(ws.EnvDir, 0755)
	return ws
}

// --- buildInfraVolumeMap ---

func TestBuildInfraVolumeMap(t *testing.T) {
	projectDir := t.TempDir()

	tests := []struct {
		name      string
		deps      *config.Deps
		workspace string
		wantKeys  []string
		wantErr   bool
	}{
		{
			name: "inline infra with named volume",
			deps: &config.Deps{
				Project: config.Project{Name: "proj"},
				Infra: map[string]config.InfraEntry{
					"pg": {Inline: &config.Infra{
						Image:   "postgres",
						Volumes: []string{"pgdata:/var/lib/postgresql/data"},
					}},
				},
			},
			workspace: "ws",
			wantKeys:  []string{"pgdata"},
		},
		{
			name: "external path entry skipped",
			deps: &config.Deps{
				Project: config.Project{Name: "proj"},
				Infra: map[string]config.InfraEntry{
					"ext": {Path: "infra.yml"},
				},
			},
			workspace: "ws",
			wantKeys:  []string{},
		},
		{
			name: "bind mount skipped",
			deps: &config.Deps{
				Project: config.Project{Name: "proj"},
				Infra: map[string]config.InfraEntry{
					"a": {Inline: &config.Infra{
						Image:   "nginx",
						Volumes: []string{"./data:/data"},
					}},
				},
			},
			workspace: "ws",
			wantKeys:  []string{},
		},
		{
			name: "no infra",
			deps: &config.Deps{
				Project: config.Project{Name: "proj"},
			},
			workspace: "ws",
			wantKeys:  []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := buildInfraVolumeMap(tt.deps, projectDir, tt.workspace)
			if (err != nil) != tt.wantErr {
				t.Errorf("buildInfraVolumeMap() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if len(got) != len(tt.wantKeys) {
				t.Errorf("buildInfraVolumeMap() keys = %v, want %v", got, tt.wantKeys)
			}
			for _, k := range tt.wantKeys {
				if _, ok := got[k]; !ok {
					t.Errorf("buildInfraVolumeMap() missing key %q in %v", k, got)
				}
			}
		})
	}
}

// --- buildServiceVolumeMap ---

func TestBuildServiceVolumeMap(t *testing.T) {
	projectDir := t.TempDir()

	deps := &config.Deps{
		Project: config.Project{Name: "proj"},
		Services: map[string]config.Service{
			"api": {
				Source: config.SourceConfig{Kind: "image", Image: "nginx"},
				Docker: &config.DockerConfig{
					Volumes: []string{"api-data:/data", "./src:/app"},
				},
			},
			"host-svc": {
				Source: config.SourceConfig{Kind: "git", Command: "npm start"},
			},
		},
	}

	got, err := buildServiceVolumeMap(deps, projectDir)
	if err != nil {
		t.Fatalf("buildServiceVolumeMap() error = %v", err)
	}
	if _, ok := got["api-data"]; !ok {
		t.Errorf("expected api-data in %v", got)
	}
	if len(got) != 1 {
		t.Errorf("expected 1 named volume, got %d: %v", len(got), got)
	}
}

// --- collectNormalizedVolumes ---

func TestCollectNormalizedVolumes(t *testing.T) {
	infraMap := map[string]string{
		"pgdata": "ws_pgdata",
		"shared": "ws_shared",
	}
	svcMap := map[string]string{
		"api-data": "proj_api-data",
		"shared":   "ws_shared", // duplicate name (different source key)
	}

	got := collectNormalizedVolumes(infraMap, svcMap)

	wantSet := map[string]bool{
		"ws_pgdata":     true,
		"ws_shared":     true,
		"proj_api-data": true,
	}

	if len(got) != len(wantSet) {
		t.Errorf("collectNormalizedVolumes() = %v (len %d), want len %d",
			got, len(got), len(wantSet))
	}
	for _, n := range got {
		if !wantSet[n] {
			t.Errorf("unexpected volume %q", n)
		}
	}
}

// --- buildComposeBase ---

func TestBuildComposeBase(t *testing.T) {
	tests := []struct {
		name             string
		deps             *config.Deps
		normalizedVols   []string
		wantHasVolumes   bool
		wantStaticIPIpam bool
	}{
		{
			name: "no services, no volumes",
			deps: &config.Deps{
				Project: config.Project{Name: "proj"},
				Network: config.NetworkConfig{Name: "raioz-net"},
			},
			wantHasVolumes:   false,
			wantStaticIPIpam: false,
		},
		{
			name: "with normalized volumes",
			deps: &config.Deps{
				Project: config.Project{Name: "proj"},
				Network: config.NetworkConfig{Name: "raioz-net"},
			},
			normalizedVols: []string{"proj_data"},
			wantHasVolumes: true,
		},
		{
			name: "service with static IP",
			deps: &config.Deps{
				Project: config.Project{Name: "proj"},
				Network: config.NetworkConfig{
					Name: "raioz-net", Subnet: "150.150.0.0/16", IsObject: true,
				},
				Services: map[string]config.Service{
					"api": {
						Docker: &config.DockerConfig{IP: "150.150.0.10"},
					},
				},
			},
			wantStaticIPIpam: true,
		},
		{
			name: "infra with static IP",
			deps: &config.Deps{
				Project: config.Project{Name: "proj"},
				Network: config.NetworkConfig{
					Name: "raioz-net", Subnet: "150.150.0.0/16", IsObject: true,
				},
				Infra: map[string]config.InfraEntry{
					"pg": {Inline: &config.Infra{
						Image: "postgres", IP: "150.150.0.11",
					}},
				},
			},
			wantStaticIPIpam: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compose := buildComposeBase(
				tt.deps, tt.deps.Network.GetName(), tt.normalizedVols,
			)

			// Base keys always present
			if _, ok := compose["services"]; !ok {
				t.Error("missing 'services' key")
			}
			networks, ok := compose["networks"].(map[string]any)
			if !ok {
				t.Fatal("missing 'networks' map")
			}
			netEntry, ok := networks[tt.deps.Network.GetName()].(map[string]any)
			if !ok {
				t.Fatal("missing network entry")
			}
			if netEntry["external"] != true {
				t.Error("network should be external")
			}

			_, hasVolumes := compose["volumes"]
			if hasVolumes != tt.wantHasVolumes {
				t.Errorf("hasVolumes = %v, want %v", hasVolumes, tt.wantHasVolumes)
			}

			_, hasIpam := netEntry["ipam"]
			if hasIpam != tt.wantStaticIPIpam {
				t.Errorf("hasIpam = %v, want %v", hasIpam, tt.wantStaticIPIpam)
			}
		})
	}
}

// --- marshalAndWriteCompose ---

func TestMarshalAndWriteCompose(t *testing.T) {
	tmpDir := t.TempDir()
	ws := mkWorkspace(tmpDir)

	compose := map[string]any{
		"services": map[string]any{
			"api": map[string]any{"image": "nginx"},
		},
		"networks": map[string]any{
			"mynet": map[string]any{"external": true},
		},
	}

	path, err := marshalAndWriteCompose(compose, ws)
	if err != nil {
		t.Fatalf("marshalAndWriteCompose() error = %v", err)
	}

	expectedPath := filepath.Join(tmpDir, "docker-compose.generated.yml")
	if path != expectedPath {
		t.Errorf("path = %q, want %q", path, expectedPath)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read written compose: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "auto-generated by raioz") {
		t.Error("missing header comment")
	}
	if !strings.Contains(content, "nginx") {
		t.Error("missing service image")
	}

	// Ensure YAML is re-parseable
	var parsed map[string]any
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		t.Errorf("yaml parse error: %v", err)
	}
}
