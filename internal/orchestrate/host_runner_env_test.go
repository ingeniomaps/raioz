package orchestrate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"raioz/internal/domain/interfaces"
)

// lastValue returns the value a process would see for key: the last
// KEY=VALUE entry wins, as exec does.
func lastValue(env []string, key string) (string, bool) {
	val, found := "", false
	for _, kv := range env {
		if v, ok := strings.CutPrefix(kv, key+"="); ok {
			val, found = v, true
		}
	}
	return val, found
}

func TestHostProcessEnv(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, ".env")
	if err := os.WriteFile(envFile, []byte("TAG=from-file\nSHARED=from-file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("RAIOZ_TEST_INHERITED", "yes")
	t.Setenv("TAG", "from-parent")

	tests := []struct {
		name string
		svc  interfaces.ServiceContext
		want map[string]string
		miss []string
	}{
		{
			name: "env file reaches the process and beats the inherited value",
			svc:  interfaces.ServiceContext{Name: "web", EnvFilePaths: []string{envFile}},
			want: map[string]string{"TAG": "from-file", "RAIOZ_TEST_INHERITED": "yes"},
		},
		{
			name: "computed vars beat the env file",
			svc: interfaces.ServiceContext{
				Name:         "web",
				EnvFilePaths: []string{envFile},
				EnvVars:      map[string]string{"SHARED": "computed", "PORT": "3001"},
			},
			want: map[string]string{"TAG": "from-file", "SHARED": "computed", "PORT": "3001"},
		},
		{
			name: "missing env file is skipped, the rest still applies",
			svc: interfaces.ServiceContext{
				Name:         "web",
				EnvFilePaths: []string{filepath.Join(dir, "absent.env"), envFile},
				EnvVars:      map[string]string{"PORT": "3001"},
			},
			want: map[string]string{"TAG": "from-file", "PORT": "3001"},
		},
		{
			name: "no env file keeps the inherited value",
			svc:  interfaces.ServiceContext{Name: "web"},
			want: map[string]string{"TAG": "from-parent"},
			miss: []string{"SHARED"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := hostProcessEnv(tc.svc)
			for k, want := range tc.want {
				if v, _ := lastValue(got, k); v != want {
					t.Errorf("%s = %q, want %q", k, v, want)
				}
			}
			for _, k := range tc.miss {
				if v, ok := lastValue(got, k); ok {
					t.Errorf("%s = %q, want it unset", k, v)
				}
			}
		})
	}
}
