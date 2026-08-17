package models

import (
	"encoding/json"
	"testing"

	"gopkg.in/yaml.v3"
)

// A dependency is either a path to a compose fragment or an inline spec.
// Path and Inline are mutually exclusive, and every consumer branches on
// which one is set.
func TestInfraEntryUnmarshalJSON(t *testing.T) {
	cases := []struct {
		name       string
		input      string
		wantErr    bool
		wantPath   string
		wantInline bool
		wantImage  string
	}{
		{name: "path string", input: `"./infra/db.yml"`, wantPath: "./infra/db.yml"},
		{
			name:       "inline spec",
			input:      `{"image": "postgres", "tag": "16"}`,
			wantInline: true,
			wantImage:  "postgres",
		},
		{name: "number is neither shape", input: `42`, wantErr: true},
		{
			// Empty object is a valid inline spec with nothing set; the
			// validator downstream is what rejects an image-less dep.
			name:       "empty object is an inline spec",
			input:      `{}`,
			wantInline: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got InfraEntry
			err := json.Unmarshal([]byte(tc.input), &got)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got %+v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if got.Path != tc.wantPath {
				t.Errorf("Path = %q, want %q", got.Path, tc.wantPath)
			}
			if (got.Inline != nil) != tc.wantInline {
				t.Errorf("Inline set = %v, want %v", got.Inline != nil, tc.wantInline)
			}
			if tc.wantImage != "" && got.Inline.Image != tc.wantImage {
				t.Errorf("Inline.Image = %q, want %q", got.Inline.Image, tc.wantImage)
			}
		})
	}
}

func TestInfraEntryMarshalJSON(t *testing.T) {
	cases := []struct {
		name string
		in   InfraEntry
		want string
	}{
		{name: "path", in: InfraEntry{Path: "./db.yml"}, want: `"./db.yml"`},
		{
			name: "inline",
			in:   InfraEntry{Inline: &Infra{Image: "redis"}},
			want: `{"image":"redis"}`,
		},
		{name: "neither", in: InfraEntry{}, want: `null`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := json.Marshal(tc.in)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			if string(got) != tc.want {
				t.Errorf("got %s, want %s", got, tc.want)
			}
		})
	}
}

// Profile filtering only applies to inline deps: a compose fragment has no
// profiles of its own, so it must never be filtered out by one.
func TestInfraEntryProfiles(t *testing.T) {
	inline := InfraEntry{Inline: &Infra{Profiles: []string{"backend"}}}
	if got := inline.Profiles(); len(got) != 1 || got[0] != "backend" {
		t.Errorf("inline Profiles() = %v, want [backend]", got)
	}
	path := InfraEntry{Path: "./db.yml"}
	if got := path.Profiles(); got != nil {
		t.Errorf("path-based entry must carry no profiles, got %v", got)
	}
}

// `proxy:` is either a bool or an object. The object form implies
// enabled — a user who spelled out a domain clearly wants the proxy on.
func TestProxyConfigUnmarshalYAML(t *testing.T) {
	cases := []struct {
		name        string
		input       string
		wantEnabled bool
		wantDomain  string
	}{
		{name: "true", input: "true", wantEnabled: true},
		{name: "false", input: "false"},
		{
			name:        "object implies enabled",
			input:       "domain: acme.dev",
			wantEnabled: true,
			wantDomain:  "acme.dev",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got ProxyConfig
			if err := yaml.Unmarshal([]byte(tc.input), &got); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if got.Enabled != tc.wantEnabled {
				t.Errorf("Enabled = %v, want %v", got.Enabled, tc.wantEnabled)
			}
			if got.Domain != tc.wantDomain {
				t.Errorf("Domain = %q, want %q", got.Domain, tc.wantDomain)
			}
		})
	}
}

func TestProxyConfigUnmarshalYAMLRejectsGarbage(t *testing.T) {
	var got ProxyConfig
	if err := yaml.Unmarshal([]byte("[1, 2]"), &got); err == nil {
		t.Errorf("a sequence is neither shape; expected an error, got %+v", got)
	}
}

// The workspace is what scopes the Docker network and shared deps. It
// falls back to the project name, and consumers need to tell an explicit
// workspace from that fallback (shared deps hinge on it — ADR-002).
func TestDepsWorkspaceName(t *testing.T) {
	explicit := &Deps{Workspace: "acme", Project: Project{Name: "shop"}}
	if got := explicit.GetWorkspaceName(); got != "acme" {
		t.Errorf("GetWorkspaceName() = %q, want acme", got)
	}
	if !explicit.HasExplicitWorkspace() {
		t.Error("HasExplicitWorkspace() = false for a declared workspace")
	}

	implicit := &Deps{Project: Project{Name: "shop"}}
	if got := implicit.GetWorkspaceName(); got != "shop" {
		t.Errorf("GetWorkspaceName() = %q, want the project name", got)
	}
	if implicit.HasExplicitWorkspace() {
		t.Error("HasExplicitWorkspace() = true without a declared workspace")
	}
}
