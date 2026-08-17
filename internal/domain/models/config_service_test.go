package models

import (
	"testing"

	"gopkg.in/yaml.v3"
)

// dependsOn can be declared at the service level or inside docker:. Both
// feed ordering, compose depends_on and validation, so the merge has to
// honor both and emit each name once.
func TestServiceGetDependsOn(t *testing.T) {
	cases := []struct {
		name string
		svc  Service
		want []string
	}{
		{name: "none", svc: Service{}},
		{
			name: "service level only",
			svc:  Service{DependsOn: []string{"db", "cache"}},
			want: []string{"db", "cache"},
		},
		{
			name: "docker level only",
			svc:  Service{Docker: &DockerConfig{DependsOn: []string{"db"}}},
			want: []string{"db"},
		},
		{
			name: "merged, service order first",
			svc: Service{
				DependsOn: []string{"db"},
				Docker:    &DockerConfig{DependsOn: []string{"cache"}},
			},
			want: []string{"db", "cache"},
		},
		{
			name: "declared in both places counts once",
			svc: Service{
				DependsOn: []string{"db", "cache"},
				Docker:    &DockerConfig{DependsOn: []string{"db"}},
			},
			want: []string{"db", "cache"},
		},
		{
			name: "duplicates within one level collapse",
			svc:  Service{DependsOn: []string{"db", "db"}},
			want: []string{"db"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.svc.GetDependsOn()
			if len(got) != len(tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Errorf("[%d] = %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

// `watch:` accepts true/false or "native" (the service reloads itself).
// Mode is what decides whether raioz runs its own watcher.
func TestYAMLWatchUnmarshalYAML(t *testing.T) {
	cases := []struct {
		name        string
		input       string
		wantEnabled bool
		wantMode    string
	}{
		{name: "true", input: "true", wantEnabled: true},
		{name: "false", input: "false"},
		{name: "native", input: "native", wantEnabled: true, wantMode: "native"},
		{
			// Neither shape: the unmarshaler swallows it and leaves the
			// zero value rather than failing the whole config.
			name:  "mapping is ignored",
			input: "{a: 1}",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got YAMLWatch
			if err := yaml.Unmarshal([]byte(tc.input), &got); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if got.Enabled != tc.wantEnabled {
				t.Errorf("Enabled = %v, want %v", got.Enabled, tc.wantEnabled)
			}
			if got.Mode != tc.wantMode {
				t.Errorf("Mode = %q, want %q", got.Mode, tc.wantMode)
			}
		})
	}
}

// A feature flag decides whether a service exists in this run at all, so
// each input has to win in the documented order: disabled beats
// everything, then the explicit enable, then the env var, then profiles.
func TestFeatureFlagIsEnabled(t *testing.T) {
	cases := []struct {
		name    string
		flag    FeatureFlagConfig
		profile string
		envVars map[string]string
		want    bool
	}{
		{name: "empty config defaults to enabled", want: true},
		{
			name: "disabled wins over enabled",
			flag: FeatureFlagConfig{Enabled: true, Disabled: true},
		},
		{name: "enabled", flag: FeatureFlagConfig{Enabled: true}, want: true},
		{
			name:    "enabled but scoped to another profile",
			flag:    FeatureFlagConfig{Enabled: true, Profiles: []string{"backend"}},
			profile: "frontend",
		},
		{
			name:    "enabled and scoped to this profile",
			flag:    FeatureFlagConfig{Enabled: true, Profiles: []string{"backend"}},
			profile: "backend",
			want:    true,
		},
		{
			name:    "env var must equal the declared value",
			flag:    FeatureFlagConfig{EnvVar: "FEATURE", EnvValue: "on"},
			envVars: map[string]string{"FEATURE": "on"},
			want:    true,
		},
		{
			name:    "env var with the wrong value",
			flag:    FeatureFlagConfig{EnvVar: "FEATURE", EnvValue: "on"},
			envVars: map[string]string{"FEATURE": "off"},
		},
		{
			name:    "bare env var: any truthy value",
			flag:    FeatureFlagConfig{EnvVar: "FEATURE"},
			envVars: map[string]string{"FEATURE": "1"},
			want:    true,
		},
		{
			name:    `bare env var: "false" reads as off`,
			flag:    FeatureFlagConfig{EnvVar: "FEATURE"},
			envVars: map[string]string{"FEATURE": "false"},
		},
		{
			name:    `bare env var: "0" reads as off`,
			flag:    FeatureFlagConfig{EnvVar: "FEATURE"},
			envVars: map[string]string{"FEATURE": "0"},
		},
		{
			name: "bare env var absent everywhere",
			flag: FeatureFlagConfig{EnvVar: "RAIOZ_FLAG_NOT_SET_ANYWHERE"},
		},
		{
			name:    "profiles only, matching",
			flag:    FeatureFlagConfig{Profiles: []string{"backend"}},
			profile: "backend",
			want:    true,
		},
		{
			// Profile-scoped and not matching still falls through to the
			// default-enabled tail.
			name:    "profiles only, not matching",
			flag:    FeatureFlagConfig{Profiles: []string{"backend"}},
			profile: "frontend",
			want:    true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.flag.IsEnabled(tc.profile, tc.envVars); got != tc.want {
				t.Errorf("IsEnabled(%q) = %v, want %v", tc.profile, got, tc.want)
			}
		})
	}
}

// The env var falls back to the process environment when the caller's map
// doesn't carry it — that is how `FEATURE=1 raioz up` works.
func TestFeatureFlagReadsProcessEnv(t *testing.T) {
	t.Setenv("RAIOZ_TEST_FEATURE", "1")
	flag := FeatureFlagConfig{EnvVar: "RAIOZ_TEST_FEATURE"}
	if !flag.IsEnabled("", nil) {
		t.Error("expected the process environment to satisfy the flag")
	}
}
