package docker

import (
	"strings"
	"testing"
)

func TestNormalizeVolumeName(t *testing.T) {
	tests := []struct {
		name     string
		project  string
		volume   string
		want     string
		wantErr  bool
		wantLen  int // 0 means no length check
		hasError bool
	}{
		{
			name:    "simple",
			project: "proj",
			volume:  "data",
			want:    "proj_data",
		},
		{
			name:    "uppercase normalization",
			project: "Proj",
			volume:  "DATA",
			want:    "proj_data",
		},
		{
			name:    "preserves underscores",
			project: "proj",
			volume:  "my_data",
			want:    "proj_my_data",
		},
		{
			name:    "already prefixed",
			project: "proj",
			volume:  "proj_mydata",
			want:    "proj_mydata",
		},
		{
			name:    "invalid chars normalized",
			project: "proj",
			volume:  "my@data",
			want:    "proj_my-data",
		},
		{
			name:    "empty volume",
			project: "proj",
			volume:  "",
			wantErr: true,
		},
		{
			name:    "empty project",
			project: "",
			volume:  "data",
			wantErr: true,
		},
		{
			name:    "very long volume is truncated",
			project: "proj",
			volume:  strings.Repeat("a", 300),
			wantLen: MaxVolumeNameLength,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeVolumeName(tt.project, tt.volume)
			if (err != nil) != tt.wantErr {
				t.Errorf("NormalizeVolumeName err = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if tt.want != "" && got != tt.want {
				t.Errorf("NormalizeVolumeName = %q, want %q", got, tt.want)
			}
			if tt.wantLen > 0 && len(got) > tt.wantLen {
				t.Errorf("NormalizeVolumeName len = %d, max %d", len(got), tt.wantLen)
			}
		})
	}
}

func TestNormalizeInfraName(t *testing.T) {
	// NormalizeInfraName is a wrapper around NormalizeContainerName
	got, err := NormalizeInfraName("ws", "pg", "proj", true)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got != "ws-pg" {
		t.Errorf("got %q, want ws-pg", got)
	}

	got2, err := NormalizeInfraName("proj", "pg", "proj", false)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got2 != "raioz-proj-pg" {
		t.Errorf("got %q, want raioz-proj-pg", got2)
	}
}

func TestNormalizeVolumeNamesInStrings(t *testing.T) {
	volumeMap := map[string]string{}
	got, err := NormalizeVolumeNamesInStrings(
		[]string{"data:/data", "./src:/app", "cache:/cache"},
		"proj", volumeMap,
	)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(got) != 3 {
		t.Errorf("got %v", got)
	}
	// First is named and should be normalized
	if !strings.HasPrefix(got[0], "proj_data") {
		t.Errorf("got[0] = %q, expected proj_data:", got[0])
	}
	// Bind mount preserved
	if got[1] != "./src:/app" {
		t.Errorf("got[1] = %q, want ./src:/app", got[1])
	}
	// volumeMap populated
	if volumeMap["data"] == "" {
		t.Error("volumeMap not populated for data")
	}
	if volumeMap["cache"] == "" {
		t.Error("volumeMap not populated for cache")
	}
}
