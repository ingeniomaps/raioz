package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- bridge ----------------------------------------------------------------

func TestYAMLToDeps_DepWithProject(t *testing.T) {
	cfg := &RaiozConfig{
		Project: "accounts",
		Deps: map[string]YAMLDependency{
			"keycloak": {
				Project:          "/abs/path/keycloak",
				RequiredHostname: "sso",
			},
		},
	}
	deps, err := YAMLToDeps(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := deps.Infra["keycloak"].Inline
	if got.Project != "/abs/path/keycloak" {
		t.Errorf("Project not bridged: got %q", got.Project)
	}
	if got.RequiredHostname != "sso" {
		t.Errorf("RequiredHostname not bridged: got %q", got.RequiredHostname)
	}
	if got.Image != "" || len(got.Compose) > 0 {
		t.Errorf("project-only dep should have no image/compose: %+v", got)
	}
}

func TestYAMLToDeps_DepWithSiblingProjectFallback(t *testing.T) {
	cfg := &RaiozConfig{
		Project: "accounts",
		Deps: map[string]YAMLDependency{
			"keycloak": {
				Image:          "keycloak:24",
				SiblingProject: "/abs/path/keycloak",
			},
		},
	}
	deps, err := YAMLToDeps(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := deps.Infra["keycloak"].Inline
	if got.SiblingProject != "/abs/path/keycloak" {
		t.Errorf("SiblingProject not bridged: got %q", got.SiblingProject)
	}
	if got.Image != "keycloak" {
		t.Errorf("Image lost in bridge: got %q", got.Image)
	}
}

// --- validation ------------------------------------------------------------

func TestValidateSiblingDependency(t *testing.T) {
	tests := []struct {
		name      string
		dep       YAMLDependency
		wantError string
	}{
		{
			name:      "valid mode A (project only)",
			dep:       YAMLDependency{Project: "../../keycloak"},
			wantError: "",
		},
		{
			name: "valid mode B (siblingProject + image)",
			dep: YAMLDependency{
				Image:          "keycloak:24",
				SiblingProject: "../../keycloak",
			},
			wantError: "",
		},
		{
			name: "project + siblingProject is rejected",
			dep: YAMLDependency{
				Project:        "../../keycloak",
				SiblingProject: "../../keycloak",
			},
			wantError: "mutually exclusive",
		},
		{
			name: "project + image is rejected",
			dep: YAMLDependency{
				Project: "../../keycloak",
				Image:   "keycloak:24",
			},
			wantError: "mutually exclusive with 'image:'",
		},
		{
			name: "project + compose is rejected",
			dep: YAMLDependency{
				Project: "../../keycloak",
				Compose: YAMLStringSlice{"./kc.yml"},
			},
			wantError: "mutually exclusive with 'image:'",
		},
		{
			name:      "siblingProject without image/compose is rejected",
			dep:       YAMLDependency{SiblingProject: "../../keycloak"},
			wantError: "siblingProject:' requires 'image:' or 'compose:'",
		},
		{
			name:      "requiredHostname without sibling is rejected",
			dep:       YAMLDependency{Image: "x:1", RequiredHostname: "sso"},
			wantError: "requiredHostname:' is only valid alongside",
		},
		{
			name: "requiredHostname with project is allowed",
			dep: YAMLDependency{
				Project:          "../../keycloak",
				RequiredHostname: "sso",
			},
			wantError: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSiblingDependency("dep", tt.dep, "raioz.yaml")
			switch {
			case tt.wantError == "" && err != nil:
				t.Errorf("expected no error, got %v", err)
			case tt.wantError != "" && err == nil:
				t.Errorf("expected error containing %q, got nil", tt.wantError)
			case tt.wantError != "" && err != nil && !strings.Contains(err.Error(), tt.wantError):
				t.Errorf("error %q does not contain %q", err.Error(), tt.wantError)
			}
		})
	}
}

// --- end-to-end through LoadYAML ------------------------------------------

func TestLoadYAML_ProjectPathIsResolvedToAbsolute(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "raioz.yaml")
	body := "project: accounts\n" +
		"dependencies:\n" +
		"  keycloak:\n" +
		"    project: ../keycloak\n"
	if err := os.WriteFile(cfgPath, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	cfg, err := LoadYAML(cfgPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := cfg.Deps["keycloak"].Project
	if !filepath.IsAbs(got) {
		t.Fatalf("expected absolute path, got %q", got)
	}
	want := filepath.Join(filepath.Dir(dir), "keycloak")
	if got != want {
		t.Errorf("resolved path = %q, want %q", got, want)
	}
}

func TestLoadYAML_RejectsProjectAndImageTogether(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "raioz.yaml")
	body := "project: accounts\n" +
		"dependencies:\n" +
		"  keycloak:\n" +
		"    project: ../keycloak\n" +
		"    image: keycloak:24\n"
	if err := os.WriteFile(cfgPath, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	_, err := LoadYAML(cfgPath)
	if err == nil || !strings.Contains(err.Error(), "mutually exclusive with 'image:'") {
		t.Errorf("expected mutual-exclusion error, got %v", err)
	}
}

func TestLoadYAML_AllowsProjectWithoutImage(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "raioz.yaml")
	body := "project: accounts\n" +
		"dependencies:\n" +
		"  keycloak:\n" +
		"    project: ../keycloak\n"
	if err := os.WriteFile(cfgPath, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	cfg, err := LoadYAML(cfgPath)
	if err != nil {
		t.Fatalf("project-only dep should be valid, got %v", err)
	}
	if cfg.Deps["keycloak"].Project == "" {
		t.Errorf("Project field empty after load")
	}
}
