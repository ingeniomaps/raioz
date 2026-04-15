package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestDetectMissingDependencies_NoMissing(t *testing.T) {
	deps := &Deps{
		Services: map[string]Service{
			"api": {DependsOn: []string{"postgres"}},
		},
		Infra: map[string]InfraEntry{
			"postgres": {Inline: &Infra{Image: "postgres:16"}},
		},
	}

	resolver := func(name string, svc Service) string { return "" }
	missing, err := DetectMissingDependencies(deps, resolver)
	if err != nil {
		t.Fatalf("DetectMissingDependencies() error = %v", err)
	}
	if len(missing) != 0 {
		t.Errorf("expected 0 missing, got %d", len(missing))
	}
}

func TestDetectMissingDependencies_MissingInfra(t *testing.T) {
	deps := &Deps{
		Services: map[string]Service{
			"api": {DependsOn: []string{"redis", "postgres"}},
		},
		Infra: map[string]InfraEntry{
			"postgres": {Inline: &Infra{Image: "postgres:16"}},
		},
	}

	resolver := func(name string, svc Service) string { return "" }
	missing, err := DetectMissingDependencies(deps, resolver)
	if err != nil {
		t.Fatalf("DetectMissingDependencies() error = %v", err)
	}
	if len(missing) != 1 {
		t.Fatalf("expected 1 missing, got %d", len(missing))
	}
	if missing[0].Dependency != "redis" {
		t.Errorf("expected redis missing, got %s", missing[0].Dependency)
	}
	if missing[0].RequiredBy != "api" {
		t.Errorf("expected RequiredBy=api, got %s", missing[0].RequiredBy)
	}
}

func TestDetectMissingDependencies_DepInServices(t *testing.T) {
	deps := &Deps{
		Services: map[string]Service{
			"api":  {DependsOn: []string{"auth"}},
			"auth": {},
		},
		Infra: map[string]InfraEntry{},
	}

	resolver := func(name string, svc Service) string { return "" }
	missing, err := DetectMissingDependencies(deps, resolver)
	if err != nil {
		t.Fatalf("DetectMissingDependencies() error = %v", err)
	}
	if len(missing) != 0 {
		t.Errorf("expected 0 missing (auth is in services), got %d", len(missing))
	}
}

func TestDetectMissingDependencies_GitServiceWithSubConfig(t *testing.T) {
	tmpDir := t.TempDir()
	svcDir := filepath.Join(tmpDir, "auth")
	if err := os.MkdirAll(svcDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Create a .raioz.json in the service directory with an extra dependency
	subConfig := map[string]interface{}{
		"project": map[string]interface{}{"name": "auth"},
		"services": map[string]interface{}{
			"worker": map[string]interface{}{
				"source": map[string]interface{}{"kind": "local"},
			},
		},
	}
	data, _ := json.Marshal(subConfig)
	if err := os.WriteFile(filepath.Join(svcDir, ".raioz.json"), data, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	deps := &Deps{
		Services: map[string]Service{
			"auth": {Source: SourceConfig{Kind: "git"}},
		},
		Infra: map[string]InfraEntry{},
	}

	resolver := func(name string, svc Service) string { return filepath.Join(tmpDir, name) }
	missing, err := DetectMissingDependencies(deps, resolver)
	if err != nil {
		t.Fatalf("DetectMissingDependencies() error = %v", err)
	}

	// "worker" from sub-config should appear as missing since it's not in root
	found := false
	for _, m := range missing {
		if m.Dependency == "worker" {
			found = true
			if m.FoundPath == "" {
				t.Error("expected FoundPath to be set for sub-config dependency")
			}
		}
	}
	if !found {
		t.Error("expected 'worker' to be detected as missing from sub-config")
	}
}

func TestDetectMissingDependencies_SkipsNonGitServices(t *testing.T) {
	tmpDir := t.TempDir()
	svcDir := filepath.Join(tmpDir, "api")
	os.MkdirAll(svcDir, 0o755)
	subConfig := map[string]interface{}{
		"project": map[string]interface{}{"name": "api"},
		"services": map[string]interface{}{
			"extra": map[string]interface{}{
				"source": map[string]interface{}{"kind": "local"},
			},
		},
	}
	data, _ := json.Marshal(subConfig)
	os.WriteFile(filepath.Join(svcDir, ".raioz.json"), data, 0o644)

	deps := &Deps{
		Services: map[string]Service{
			"api": {Source: SourceConfig{Kind: "local"}}, // not git
		},
		Infra: map[string]InfraEntry{},
	}

	resolver := func(name string, svc Service) string { return filepath.Join(tmpDir, name) }
	missing, err := DetectMissingDependencies(deps, resolver)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	// Should not scan sub-config because kind is "local", not "git"
	if len(missing) != 0 {
		t.Errorf("expected 0 missing for non-git service, got %d", len(missing))
	}
}

func TestFindServiceConfig_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	_, _, err := FindServiceConfig(tmpDir)
	if err == nil {
		t.Error("expected error when .raioz.json does not exist")
	}
}

func TestFindServiceConfig_Found(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := map[string]interface{}{
		"project":  map[string]interface{}{"name": "test"},
		"services": map[string]interface{}{},
	}
	data, _ := json.Marshal(cfg)
	os.WriteFile(filepath.Join(tmpDir, ".raioz.json"), data, 0o644)

	deps, path, err := FindServiceConfig(tmpDir)
	if err != nil {
		t.Fatalf("FindServiceConfig() error = %v", err)
	}
	if deps == nil {
		t.Error("expected non-nil deps")
	}
	if path == "" {
		t.Error("expected non-empty path")
	}
	if deps.Project.Name != "test" {
		t.Errorf("project name = %q, want %q", deps.Project.Name, "test")
	}
}

func TestFindServiceConfig_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, ".raioz.json"), []byte("{invalid"), 0o644)

	_, _, err := FindServiceConfig(tmpDir)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestDetectDependencyConflicts_NoConflicts(t *testing.T) {
	deps := &Deps{
		Services: map[string]Service{
			"api": {Source: SourceConfig{Kind: "local"}},
		},
	}
	resolver := func(name string, svc Service) string { return "" }
	conflicts, err := DetectDependencyConflicts(deps, resolver)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(conflicts) != 0 {
		t.Errorf("expected 0 conflicts, got %d", len(conflicts))
	}
}

func TestDetectDependencyConflicts_BranchConflict(t *testing.T) {
	tmpDir := t.TempDir()
	svcDir := filepath.Join(tmpDir, "api")
	os.MkdirAll(svcDir, 0o755)

	subConfig := map[string]interface{}{
		"project": map[string]interface{}{"name": "sub"},
		"services": map[string]interface{}{
			"api": map[string]interface{}{
				"source": map[string]interface{}{
					"kind":   "git",
					"repo":   "github.com/test/api",
					"branch": "develop",
				},
			},
		},
	}
	data, _ := json.Marshal(subConfig)
	os.WriteFile(filepath.Join(svcDir, ".raioz.json"), data, 0o644)

	deps := &Deps{
		Services: map[string]Service{
			"api": {Source: SourceConfig{
				Kind:   "git",
				Repo:   "github.com/test/api",
				Branch: "main",
			}},
		},
	}

	resolver := func(name string, svc Service) string { return filepath.Join(tmpDir, name) }
	conflicts, err := DetectDependencyConflicts(deps, resolver)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(conflicts) != 1 {
		t.Fatalf("expected 1 conflict, got %d", len(conflicts))
	}
	if conflicts[0].ServiceName != "api" {
		t.Errorf("conflict service = %q, want %q", conflicts[0].ServiceName, "api")
	}

	foundBranch := false
	for _, d := range conflicts[0].Differences {
		if len(d) > 6 && d[:6] == "branch" {
			foundBranch = true
		}
	}
	if !foundBranch {
		t.Errorf("expected branch difference, got %v", conflicts[0].Differences)
	}
}

func TestDetectDependencyConflicts_DockerConflict(t *testing.T) {
	tmpDir := t.TempDir()
	svcDir := filepath.Join(tmpDir, "api")
	os.MkdirAll(svcDir, 0o755)

	subConfig := map[string]interface{}{
		"project": map[string]interface{}{"name": "sub"},
		"services": map[string]interface{}{
			"api": map[string]interface{}{
				"source": map[string]interface{}{"kind": "git"},
				"docker": map[string]interface{}{
					"mode":  "prod",
					"ports": []string{"8080:8080"},
				},
			},
		},
	}
	data, _ := json.Marshal(subConfig)
	os.WriteFile(filepath.Join(svcDir, ".raioz.json"), data, 0o644)

	deps := &Deps{
		Services: map[string]Service{
			"api": {
				Source: SourceConfig{Kind: "git"},
				Docker: &DockerConfig{
					Mode:  "dev",
					Ports: []string{"3000:3000"},
				},
			},
		},
	}

	resolver := func(name string, svc Service) string { return filepath.Join(tmpDir, name) }
	conflicts, err := DetectDependencyConflicts(deps, resolver)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(conflicts) == 0 {
		t.Fatal("expected at least 1 conflict for docker mode difference")
	}
}

func TestDetectDependencyConflicts_OneHasDocker(t *testing.T) {
	tmpDir := t.TempDir()
	svcDir := filepath.Join(tmpDir, "api")
	os.MkdirAll(svcDir, 0o755)

	subConfig := map[string]interface{}{
		"project": map[string]interface{}{"name": "sub"},
		"services": map[string]interface{}{
			"api": map[string]interface{}{
				"source": map[string]interface{}{"kind": "git"},
				// no docker
			},
		},
	}
	data, _ := json.Marshal(subConfig)
	os.WriteFile(filepath.Join(svcDir, ".raioz.json"), data, 0o644)

	deps := &Deps{
		Services: map[string]Service{
			"api": {
				Source: SourceConfig{Kind: "git"},
				Docker: &DockerConfig{Mode: "dev"},
			},
		},
	}

	resolver := func(name string, svc Service) string { return filepath.Join(tmpDir, name) }
	conflicts, err := DetectDependencyConflicts(deps, resolver)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(conflicts) == 0 {
		t.Fatal("expected conflict when one has docker and the other does not")
	}

	foundDocker := false
	for _, d := range conflicts[0].Differences {
		if len(d) >= 6 && d[:6] == "docker" {
			foundDocker = true
		}
	}
	if !foundDocker {
		t.Errorf("expected docker difference, got %v", conflicts[0].Differences)
	}
}

func TestEqualStringSlices(t *testing.T) {
	tests := []struct {
		name string
		a, b []string
		want bool
	}{
		{"both nil", nil, nil, true},
		{"both empty", []string{}, []string{}, true},
		{"equal", []string{"a", "b"}, []string{"a", "b"}, true},
		{"different length", []string{"a"}, []string{"a", "b"}, false},
		{"different content", []string{"a", "b"}, []string{"a", "c"}, false},
		{"one nil one empty", nil, []string{}, true},
		{"single element equal", []string{"x"}, []string{"x"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := equalStringSlices(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("equalStringSlices(%v, %v) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}
