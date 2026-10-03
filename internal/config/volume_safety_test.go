package config

import (
	"strings"
	"testing"
)

func TestSystemVolumeWarnings(t *testing.T) {
	tests := []struct {
		name    string
		volumes []string
		want    string // substring of the single warning; "" = none
	}{
		{"named volume", []string{"pgdata:/var/lib/postgresql/data"}, ""},
		{"bind inside the project", []string{"./data:/data"}, ""},
		{"bind elsewhere in home", []string{"/home/dev/shared:/shared:ro"}, ""},
		{"system dir", []string{"/etc:/hostetc"}, "/etc"},
		{"path under a system dir", []string{"/root/.ssh:/keys:ro"}, "/root/.ssh"},
		{"relative path that climbs into one", []string{"../../etc:/x"}, "/etc"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &RaiozConfig{Deps: map[string]YAMLDependency{
				"db": {Image: "postgres:16", Volumes: tt.volumes},
			}}
			got := systemVolumeWarnings(cfg, "/work/proj")
			if tt.want == "" {
				if len(got) != 0 {
					t.Fatalf("want no warning, got %v", got)
				}
				return
			}
			if len(got) != 1 || !strings.Contains(got[0], tt.want) || !strings.Contains(got[0], "db") {
				t.Fatalf("want one warning naming db and %s, got %v", tt.want, got)
			}
		})
	}
}
