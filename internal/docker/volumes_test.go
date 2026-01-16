package docker

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestParseVolume(t *testing.T) {
	tests := []struct {
		name     string
		volume   string
		wantType VolumeType
		wantSrc  string
		wantDest string
	}{
		{
			name:     "named volume",
			volume:   "mongo-data:/data/db",
			wantType: VolumeTypeNamed,
			wantSrc:  "mongo-data",
			wantDest: "/data/db",
		},
		{
			name:     "bind mount relative",
			volume:   "./data:/app/data",
			wantType: VolumeTypeBind,
			wantSrc:  "./data",
			wantDest: "/app/data",
		},
		{
			name:     "bind mount absolute",
			volume:   "/host/path:/container/path",
			wantType: VolumeTypeBind,
			wantSrc:  "/host/path",
			wantDest: "/container/path",
		},
		{
			name:     "anonymous volume",
			volume:   "/container/path",
			wantType: VolumeTypeAnonymous,
			wantSrc:  "",
			wantDest: "/container/path",
		},
		{
			name:     "named volume with underscore",
			volume:   "my_volume:/data",
			wantType: VolumeTypeNamed,
			wantSrc:  "my_volume",
			wantDest: "/data",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := ParseVolume(tt.volume)
			if err != nil {
				t.Fatalf("ParseVolume() error = %v", err)
			}

			if info.Type != tt.wantType {
				t.Errorf("ParseVolume() Type = %v, want %v", info.Type, tt.wantType)
			}

			if info.Source != tt.wantSrc {
				t.Errorf("ParseVolume() Source = %v, want %v", info.Source, tt.wantSrc)
			}

			if info.Destination != tt.wantDest {
				t.Errorf("ParseVolume() Destination = %v, want %v", info.Destination, tt.wantDest)
			}
		})
	}
}

func TestExtractNamedVolumes(t *testing.T) {
	tests := []struct {
		name     string
		volumes  []string
		want     []string
		wantErr  bool
	}{
		{
			name:    "mixed volumes",
			volumes: []string{"mongo-data:/data/db", "./app:/app", "redis-data:/data"},
			want:    []string{"mongo-data", "redis-data"},
			wantErr: false,
		},
		{
			name:    "only bind mounts",
			volumes: []string{"./data:/app", "/host:/container"},
			want:    []string{},
			wantErr: false,
		},
		{
			name:    "only named volumes",
			volumes: []string{"volume1:/data1", "volume2:/data2"},
			want:    []string{"volume1", "volume2"},
			wantErr: false,
		},
		{
			name:    "duplicate named volumes",
			volumes: []string{"mongo-data:/data1", "mongo-data:/data2"},
			want:    []string{"mongo-data"},
			wantErr: false,
		},
		{
			name:    "empty list",
			volumes: []string{},
			want:    []string{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExtractNamedVolumes(tt.volumes)
			if (err != nil) != tt.wantErr {
				t.Errorf("ExtractNamedVolumes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Convert to map for comparison (order doesn't matter)
			gotMap := make(map[string]bool)
			for _, v := range got {
				gotMap[v] = true
			}

			wantMap := make(map[string]bool)
			for _, v := range tt.want {
				wantMap[v] = true
			}

			if len(gotMap) != len(wantMap) {
				t.Errorf("ExtractNamedVolumes() = %v, want %v", got, tt.want)
				return
			}

			for vol := range wantMap {
				if !gotMap[vol] {
					t.Errorf("ExtractNamedVolumes() missing volume %s", vol)
				}
			}
		})
	}
}

func TestVolumeExists(t *testing.T) {
	// Test with non-existent volume
	exists, err := VolumeExists("nonexistent-volume-12345")
	if err != nil {
		t.Fatalf("VolumeExists() error = %v", err)
	}
	if exists {
		t.Error("VolumeExists() should return false for non-existent volume")
	}
}

func TestGetVolumeProjects(t *testing.T) {
	tmpDir := t.TempDir()
	workspacesDir := filepath.Join(tmpDir, "workspaces")

	// Create test workspaces
	testProject1 := filepath.Join(workspacesDir, "project1")
	testProject2 := filepath.Join(workspacesDir, "project2")

	if err := os.MkdirAll(testProject1, 0755); err != nil {
		t.Fatalf("Failed to create test project1: %v", err)
	}
	if err := os.MkdirAll(testProject2, 0755); err != nil {
		t.Fatalf("Failed to create test project2: %v", err)
	}

	// Create state file for project1 with named volume "mongo-data"
	state1 := map[string]any{
		"services": map[string]any{},
		"infra": map[string]any{
			"mongo": map[string]any{
				"volumes": []string{"mongo-data:/data/db"},
			},
		},
	}
	state1Data, _ := json.Marshal(state1)
	os.WriteFile(filepath.Join(testProject1, ".state.json"), state1Data, 0644)

	// Create state file for project2 with different volume
	state2 := map[string]any{
		"infra": map[string]any{
			"redis": map[string]any{
				"volumes": []string{"redis-data:/data"},
			},
		},
	}
	state2Data, _ := json.Marshal(state2)
	os.WriteFile(filepath.Join(testProject2, ".state.json"), state2Data, 0644)

	// Test finding projects using "mongo-data"
	projects, err := GetVolumeProjects("mongo-data", tmpDir)
	if err != nil {
		t.Fatalf("GetVolumeProjects() error = %v", err)
	}

	if len(projects) != 1 {
		t.Errorf("GetVolumeProjects() found %d projects, want 1", len(projects))
	}

	if len(projects) > 0 && projects[0] != "project1" {
		t.Errorf("GetVolumeProjects() found project %s, want project1", projects[0])
	}
}
