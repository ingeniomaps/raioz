package docker

import (
	"reflect"
	"strings"
	"testing"

	"raioz/internal/config"
)

func TestDetectSharedVolumes(t *testing.T) {
	tests := []struct {
		name     string
		services map[string]ServiceVolumes
		want     map[string][]string
	}{
		{
			name:     "no_shared_volumes",
			services: map[string]ServiceVolumes{},
			want:     map[string][]string{},
		},
		{
			name: "single_service_single_volume",
			services: map[string]ServiceVolumes{
				"service-a": {
					NamedVolumes: []string{"volume-a"},
				},
			},
			want: map[string][]string{},
		},
		{
			name: "two_services_different_volumes",
			services: map[string]ServiceVolumes{
				"service-a": {
					NamedVolumes: []string{"volume-a"},
				},
				"service-b": {
					NamedVolumes: []string{"volume-b"},
				},
			},
			want: map[string][]string{},
		},
		{
			name: "two_services_shared_volume",
			services: map[string]ServiceVolumes{
				"service-a": {
					NamedVolumes: []string{"shared-volume"},
				},
				"service-b": {
					NamedVolumes: []string{"shared-volume"},
				},
			},
			want: map[string][]string{
				"shared-volume": {"service-a", "service-b"},
			},
		},
		{
			name: "three_services_shared_volume",
			services: map[string]ServiceVolumes{
				"service-a": {
					NamedVolumes: []string{"shared-volume"},
				},
				"service-b": {
					NamedVolumes: []string{"shared-volume"},
				},
				"service-c": {
					NamedVolumes: []string{"shared-volume"},
				},
			},
			want: map[string][]string{
				"shared-volume": {"service-a", "service-b", "service-c"},
			},
		},
		{
			name: "multiple_shared_volumes",
			services: map[string]ServiceVolumes{
				"service-a": {
					NamedVolumes: []string{"volume-1", "volume-2"},
				},
				"service-b": {
					NamedVolumes: []string{"volume-1", "volume-3"},
				},
				"service-c": {
					NamedVolumes: []string{"volume-2", "volume-3"},
				},
			},
			want: map[string][]string{
				"volume-1": {"service-a", "service-b"},
				"volume-2": {"service-a", "service-c"},
				"volume-3": {"service-b", "service-c"},
			},
		},
		{
			name: "service_with_multiple_volumes",
			services: map[string]ServiceVolumes{
				"service-a": {
					NamedVolumes: []string{"volume-1", "volume-2"},
				},
				"service-b": {
					NamedVolumes: []string{"volume-1"},
				},
			},
			want: map[string][]string{
				"volume-1": {"service-a", "service-b"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectSharedVolumes(tt.services)

			// Check that all expected volumes are present
			for volName, expectedServices := range tt.want {
				gotServices, exists := got[volName]
				if !exists {
					t.Errorf("DetectSharedVolumes() missing volume %s", volName)
					continue
				}

				// Check that service lists match (order may differ, so use reflect.DeepEqual after sorting)
				if !reflect.DeepEqual(gotServices, expectedServices) {
					t.Errorf("DetectSharedVolumes() for volume %s = %v, want %v", volName, gotServices, expectedServices)
				}
			}

			// Check that no unexpected volumes are present
			for volName := range got {
				if _, exists := tt.want[volName]; !exists {
					t.Errorf("DetectSharedVolumes() unexpected volume %s", volName)
				}
			}

			// Check total count
			if len(got) != len(tt.want) {
				t.Errorf("DetectSharedVolumes() returned %d volumes, want %d", len(got), len(tt.want))
			}
		})
	}
}

func TestFormatSharedVolumesWarning(t *testing.T) {
	tests := []struct {
		name          string
		sharedVolumes map[string][]string
		wantEmpty     bool
		wantContains  []string
	}{
		{
			name:          "empty",
			sharedVolumes: map[string][]string{},
			wantEmpty:     true,
		},
		{
			name: "single_shared_volume",
			sharedVolumes: map[string][]string{
				"shared-data": {"service-a", "service-b"},
			},
			wantEmpty:    false,
			wantContains: []string{"shared-data", "service-a", "service-b", "Warning", "Note"},
		},
		{
			name: "multiple_shared_volumes",
			sharedVolumes: map[string][]string{
				"volume-1": {"service-a", "service-b"},
				"volume-2": {"service-c", "service-d"},
			},
			wantEmpty:    false,
			wantContains: []string{"volume-1", "volume-2", "service-a", "service-b", "service-c", "service-d"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatSharedVolumesWarning(tt.sharedVolumes)

			if tt.wantEmpty {
				if got != "" {
					t.Errorf("FormatSharedVolumesWarning() = %q, want empty string", got)
				}
				return
			}

			if got == "" {
				t.Error("FormatSharedVolumesWarning() returned empty string, expected warning")
				return
			}

			// Check that all expected strings are contained
			for _, wantStr := range tt.wantContains {
				if !strings.Contains(got, wantStr) {
					t.Errorf("FormatSharedVolumesWarning() output doesn't contain %q", wantStr)
				}
			}
		})
	}
}

func TestBuildServiceVolumesMap(t *testing.T) {
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
				// Docker nil -> skipped
			},
			"no-vols": {
				Source: config.SourceConfig{Kind: "image", Image: "busybox"},
				Docker: &config.DockerConfig{},
			},
		},
		Infra: map[string]config.InfraEntry{
			"db": {Inline: &config.Infra{
				Image:   "postgres",
				Volumes: []string{"pgdata:/var/lib/postgresql/data"},
			}},
			"ext": {Path: "external.yml"},
		},
	}

	got, err := BuildServiceVolumesMap(deps)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	// api has one named volume
	apiVols, ok := got["api"]
	if !ok {
		t.Fatal("api missing")
	}
	if len(apiVols.NamedVolumes) != 1 || apiVols.NamedVolumes[0] != "api-data" {
		t.Errorf("api named = %v", apiVols.NamedVolumes)
	}

	// db has one named volume
	dbVols, ok := got["db"]
	if !ok {
		t.Fatal("db missing")
	}
	if len(dbVols.NamedVolumes) != 1 || dbVols.NamedVolumes[0] != "pgdata" {
		t.Errorf("db named = %v", dbVols.NamedVolumes)
	}

	// host-svc (nil docker) and no-vols (no named) and ext (Path) should be absent
	if _, ok := got["host-svc"]; ok {
		t.Error("host-svc should be absent")
	}
	if _, ok := got["no-vols"]; ok {
		t.Error("no-vols should be absent")
	}
	if _, ok := got["ext"]; ok {
		t.Error("ext should be absent")
	}
}
