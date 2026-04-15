package docker

import (
	"os"
	"path/filepath"
	"testing"
)

func TestVolumeStringsFromServiceConfig(t *testing.T) {
	tests := []struct {
		name string
		svc  map[string]any
		want []string
	}{
		{
			name: "nil service",
			svc:  nil,
			want: nil,
		},
		{
			name: "no volumes key",
			svc:  map[string]any{"image": "nginx"},
			want: nil,
		},
		{
			name: "volumes as interface slice",
			svc: map[string]any{
				"volumes": []interface{}{"data:/data", "./src:/app"},
			},
			want: []string{"data:/data", "./src:/app"},
		},
		{
			name: "volumes wrong type",
			svc: map[string]any{
				"volumes": "string-instead-of-list",
			},
			want: nil,
		},
		{
			name: "volumes with non-string item",
			svc: map[string]any{
				"volumes": []interface{}{"data:/data", 42, "other:/other"},
			},
			want: []string{"data:/data", "other:/other"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := volumeStringsFromServiceConfig(tt.svc)
			if len(got) != len(tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("got[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestMergeExternalComposeFile(t *testing.T) {
	projectDir := t.TempDir()

	yamlContent := `services:
  pg:
    image: postgres:15
    volumes:
      - pgdata:/var/lib/postgresql/data
      - ./init.sql:/init.sql
  redis:
    image: redis:7
volumes:
  pgdata:
networks:
  other-net:
    driver: bridge
`
	infraPath := filepath.Join(projectDir, "infra.yml")
	if err := os.WriteFile(infraPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("write infra: %v", err)
	}

	compose := map[string]any{
		"services": map[string]any{},
		"networks": map[string]any{
			"raioz-net": map[string]any{"external": true},
		},
	}

	infraVolumeMap := map[string]string{}

	names, err := mergeExternalComposeFile(
		compose, projectDir, "infra.yml",
		"ws", "proj", "raioz-net", false,
		"db", infraVolumeMap,
	)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	if len(names) != 2 {
		t.Errorf("expected 2 service names, got %d: %v", len(names), names)
	}

	// Validate pgdata was normalized and added to infraVolumeMap
	if infraVolumeMap["pgdata"] == "" {
		t.Errorf("pgdata not normalized: %v", infraVolumeMap)
	}

	// Validate services merged
	services := compose["services"].(map[string]any)
	if _, ok := services["pg"]; !ok {
		t.Error("service 'pg' not merged")
	}
	if _, ok := services["redis"]; !ok {
		t.Error("service 'redis' not merged")
	}

	// Validate top-level volumes merged
	volumes, ok := compose["volumes"].(map[string]any)
	if !ok {
		t.Fatal("compose volumes not merged")
	}
	if _, ok := volumes[infraVolumeMap["pgdata"]]; !ok {
		t.Errorf("normalized volume %q not in compose volumes: %v",
			infraVolumeMap["pgdata"], volumes)
	}

	// pg service should have normalized container name
	pg := services["pg"].(map[string]any)
	if pg["container_name"] == nil {
		t.Error("pg container_name missing")
	}
	// Networks should be overridden
	nets, _ := pg["networks"].([]string)
	if len(nets) != 1 || nets[0] != "raioz-net" {
		t.Errorf("pg networks = %v, want [raioz-net]", pg["networks"])
	}
}

func TestMergeExternalComposeFile_NotFound(t *testing.T) {
	projectDir := t.TempDir()
	compose := map[string]any{
		"services": map[string]any{},
		"networks": map[string]any{},
	}

	_, err := mergeExternalComposeFile(
		compose, projectDir, "nonexistent.yml",
		"ws", "proj", "raioz-net", false,
		"db", map[string]string{},
	)
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestMergeExternalComposeFile_InvalidYAML(t *testing.T) {
	projectDir := t.TempDir()
	badPath := filepath.Join(projectDir, "bad.yml")
	if err := os.WriteFile(badPath, []byte("not: valid: yaml: ["), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	compose := map[string]any{
		"services": map[string]any{},
		"networks": map[string]any{},
	}

	_, err := mergeExternalComposeFile(
		compose, projectDir, "bad.yml",
		"ws", "proj", "raioz-net", false,
		"db", map[string]string{},
	)
	if err == nil {
		t.Error("expected error for invalid yaml")
	}
}
