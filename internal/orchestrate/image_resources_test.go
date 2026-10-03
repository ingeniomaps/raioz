package orchestrate

import (
	"os"
	"testing"

	"gopkg.in/yaml.v3"

	"raioz/internal/domain/models"
	"raioz/internal/naming"
)

func TestImageRunner_GenerateCompose_Resources(t *testing.T) {
	tests := []struct {
		name string
		res  *models.Resources
		want map[string]any // keys absent from want must be absent from the service
	}{
		{"no block, no cap", nil, map[string]any{}},
		{
			"memory also closes swap",
			&models.Resources{Memory: "256m"},
			map[string]any{"mem_limit": "256m", "memswap_limit": "256m"},
		},
		{"cpus only", &models.Resources{CPUs: 0.5}, map[string]any{"cpus": 0.5}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := makeImageSvc()
			svc.ProjectName = "res-" + t.Name()
			svc.Resources = tt.res
			t.Cleanup(func() { os.RemoveAll(naming.TempDir(svc.ProjectName)) })

			path, err := (&ImageRunner{}).generateCompose(svc)
			if err != nil {
				t.Fatalf("generateCompose: %v", err)
			}
			data, _ := os.ReadFile(path)
			var parsed map[string]any
			if err := yaml.Unmarshal(data, &parsed); err != nil {
				t.Fatal(err)
			}
			service := parsed["services"].(map[string]any)[svc.Name].(map[string]any)
			for _, key := range []string{"mem_limit", "memswap_limit", "cpus"} {
				got, present := service[key]
				want, wanted := tt.want[key]
				if present != wanted || (wanted && got != want) {
					t.Errorf("%s = %v (present %v), want %v (present %v)", key, got, present, want, wanted)
				}
			}
		})
	}
}

func TestLimitUpdateArgs(t *testing.T) {
	if got := limitUpdateArgs("c", nil); got != nil {
		t.Errorf("nothing declared must run nothing, got %v", got)
	}
	got := limitUpdateArgs("c", &models.Resources{Memory: "64m", CPUs: 0.5})
	want := []string{"update", "--memory", "64m", "--memory-swap", "64m", "--cpus", "0.5", "c"}
	if len(got) != len(want) {
		t.Fatalf("limitUpdateArgs = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("limitUpdateArgs = %v, want %v", got, want)
		}
	}
}
