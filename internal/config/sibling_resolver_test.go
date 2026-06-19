package config

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// --- ResolveSibling -------------------------------------------------------

func TestResolveSibling_Valid(t *testing.T) {
	dir := t.TempDir()
	body := "" +
		"workspace: hypixo\n" +
		"project: keycloak\n" +
		"services:\n" +
		"  keycloak:\n" +
		"    path: .\n" +
		"    hostname: sso\n"
	if err := os.WriteFile(filepath.Join(dir, "raioz.yaml"), []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	sib, err := ResolveSibling(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sib.Project != "keycloak" {
		t.Errorf("Project = %q, want keycloak", sib.Project)
	}
	if sib.Workspace != "hypixo" {
		t.Errorf("Workspace = %q, want hypixo", sib.Workspace)
	}
	if sib.Dir != dir {
		t.Errorf("Dir = %q, want %q", sib.Dir, dir)
	}
	if !strings.HasSuffix(sib.Path, "raioz.yaml") {
		t.Errorf("Path = %q, want suffix raioz.yaml", sib.Path)
	}
	if !sib.SiblingHasHostname("sso") {
		t.Errorf("expected hostname 'sso' in %v", sib.Hostnames)
	}
}

func TestResolveSibling_RejectsRelativePath(t *testing.T) {
	_, err := ResolveSibling("./relative")
	if err == nil || !strings.Contains(err.Error(), "must be absolute") {
		t.Errorf("expected absolute-path error, got %v", err)
	}
}

func TestResolveSibling_RejectsEmptyPath(t *testing.T) {
	_, err := ResolveSibling("")
	if err == nil || !strings.Contains(err.Error(), "empty") {
		t.Errorf("expected empty-path error, got %v", err)
	}
}

func TestResolveSibling_MissingFile(t *testing.T) {
	dir := t.TempDir()
	_, err := ResolveSibling(dir) // no raioz.yaml in dir
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected not-found error, got %v", err)
	}
}

func TestResolveSibling_MalformedYAML(t *testing.T) {
	dir := t.TempDir()
	body := "project: keycloak\nservices:\n  - this is not a map\n"
	if err := os.WriteFile(filepath.Join(dir, "raioz.yaml"), []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	_, err := ResolveSibling(dir)
	if err == nil {
		t.Fatal("expected parse error, got nil")
	}
	if !strings.Contains(err.Error(), "load sibling") {
		t.Errorf("error message lacks 'load sibling' context: %v", err)
	}
}

func TestResolveSibling_MissingProject(t *testing.T) {
	dir := t.TempDir()
	body := "workspace: hypixo\n" // no project:
	if err := os.WriteFile(filepath.Join(dir, "raioz.yaml"), []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	_, err := ResolveSibling(dir)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	// LoadYAML's own validator rejects empty project first; either
	// message is acceptable as long as the user learns the project
	// field is the problem.
	if !strings.Contains(err.Error(), "project") {
		t.Errorf("error should mention 'project', got %v", err)
	}
}

func TestResolveSibling_RejectsMetaKind(t *testing.T) {
	dir := t.TempDir()
	body := "" +
		"project: hypixo-meta\n" +
		"kind: meta\n" +
		"projects:\n" +
		"  - path: ./keycloak\n"
	if err := os.WriteFile(filepath.Join(dir, "raioz.yaml"), []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	_, err := ResolveSibling(dir)
	if err == nil || !strings.Contains(err.Error(), "meta-orchestrator") {
		t.Errorf("expected meta-orchestrator error, got %v", err)
	}
}

// --- collectSiblingHostnames ---------------------------------------------

func TestCollectSiblingHostnames_ServicesAndDeps(t *testing.T) {
	cfg := &RaiozConfig{
		Project: "x",
		Services: map[string]YAMLService{
			"api": { // no explicit hostname → fallback to key
				Path: ".",
			},
			"web": {
				Path:            ".",
				Hostname:        "www",
				HostnameAliases: YAMLStringSlice{"www2"},
			},
		},
		Deps: map[string]YAMLDependency{
			"postgres": { // image dep with default name
				Image: "postgres:16",
			},
			"redis": { // image dep with custom hostname
				Image:    "redis:7",
				Hostname: "cache",
			},
			"keycloak": { // sibling-only — no hostname claimed
				Project: "/abs/keycloak",
			},
		},
	}

	got := collectSiblingHostnames(cfg)
	sort.Strings(got)
	// `web` provides an explicit hostname `www` so the service key is
	// not used. `keycloak` is sibling-only and claims no hostname.
	want := []string{"api", "cache", "postgres", "www", "www2"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("hostnames = %v, want %v", got, want)
	}
}

func TestCollectSiblingHostnames_Deduplicates(t *testing.T) {
	cfg := &RaiozConfig{
		Project: "x",
		Services: map[string]YAMLService{
			"api": {
				Hostname:        "shared",
				HostnameAliases: YAMLStringSlice{"shared", "alias-1"},
			},
		},
	}

	got := collectSiblingHostnames(cfg)
	count := 0
	for _, h := range got {
		if h == "shared" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("'shared' appears %d times, want 1; full list %v", count, got)
	}
}

// --- ValidateSiblingWorkspace --------------------------------------------

func TestValidateSiblingWorkspace(t *testing.T) {
	tests := []struct {
		name      string
		consumer  string
		sibWS     string
		wantError string
	}{
		{
			name:      "match",
			consumer:  "hypixo",
			sibWS:     "hypixo",
			wantError: "",
		},
		{
			name:      "both empty",
			consumer:  "",
			sibWS:     "",
			wantError: "",
		},
		{
			name:      "consumer empty, sibling set",
			consumer:  "",
			sibWS:     "hypixo",
			wantError: "consumer\n declares none",
		},
		{
			name:      "sibling empty, consumer set",
			consumer:  "hypixo",
			sibWS:     "",
			wantError: "sibling project",
		},
		{
			name:      "mismatch",
			consumer:  "hypixo",
			sibWS:     "acme",
			wantError: "must share a workspace",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sib := &SiblingInfo{Project: "kc", Workspace: tt.sibWS}
			err := ValidateSiblingWorkspace(tt.consumer, sib)
			switch {
			case tt.wantError == "" && err != nil:
				t.Errorf("expected nil, got %v", err)
			case tt.wantError != "" && err == nil:
				t.Errorf("expected error %q, got nil", tt.wantError)
			}
		})
	}
}

func TestValidateSiblingWorkspace_NilSibling(t *testing.T) {
	if err := ValidateSiblingWorkspace("hypixo", nil); err == nil {
		t.Error("expected nil-sibling error, got nil")
	}
}

// --- ProxyTargets ---------------------------------------------------------

// Launcher-pattern siblings expose their container name
// via services.<n>.proxy.target. ResolveSibling now collects those so
// the docker.IsProjectActive fallback can probe by name when the
// label-based scan misses.
func TestResolveSibling_CollectsProxyTargets(t *testing.T) {
	dir := t.TempDir()
	body := "" +
		"workspace: hypixo\n" +
		"project: keycloak\n" +
		"services:\n" +
		"  keycloak:\n" +
		"    path: .\n" +
		"    command: make start\n" +
		"    proxy:\n" +
		"      target: hypixo-keycloak\n" +
		"      port: 8080\n" +
		"  admin:\n" +
		"    path: ./admin\n"
	if err := os.WriteFile(filepath.Join(dir, "raioz.yaml"), []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	sib, err := ResolveSibling(dir)
	if err != nil {
		t.Fatalf("ResolveSibling: %v", err)
	}
	if len(sib.ProxyTargets) != 1 || sib.ProxyTargets[0] != "hypixo-keycloak" {
		t.Errorf("ProxyTargets = %v, want [hypixo-keycloak]", sib.ProxyTargets)
	}
}

func TestResolveSibling_NoProxyTargetWhenAbsent(t *testing.T) {
	dir := t.TempDir()
	body := "" +
		"workspace: hypixo\n" +
		"project: api\n" +
		"services:\n" +
		"  api:\n" +
		"    path: .\n"
	if err := os.WriteFile(filepath.Join(dir, "raioz.yaml"), []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	sib, err := ResolveSibling(dir)
	if err != nil {
		t.Fatalf("ResolveSibling: %v", err)
	}
	if len(sib.ProxyTargets) != 0 {
		t.Errorf("ProxyTargets = %v, want empty", sib.ProxyTargets)
	}
}

// --- SiblingHasHostname ---------------------------------------------------

func TestSiblingHasHostname(t *testing.T) {
	sib := &SiblingInfo{Hostnames: []string{"sso", "www"}}
	if !sib.SiblingHasHostname("sso") {
		t.Error("expected hit on 'sso'")
	}
	if sib.SiblingHasHostname("missing") {
		t.Error("expected miss on 'missing'")
	}

	var nilSib *SiblingInfo
	if nilSib.SiblingHasHostname("sso") {
		t.Error("nil receiver should always miss")
	}
}
