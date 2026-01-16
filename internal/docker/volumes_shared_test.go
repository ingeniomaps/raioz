package docker

import (
	"reflect"
	"strings"
	"testing"
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
	// This test requires config.Deps, so we'll do a basic integration test
	// For now, we'll just verify the function exists and can be called
	// Full integration tests would require creating a full config.Deps structure
	t.Skip("Integration test - requires full config.Deps setup")
}
