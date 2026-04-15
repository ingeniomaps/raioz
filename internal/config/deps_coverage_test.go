package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestGetWorkspaceName_Explicit(t *testing.T) {
	deps := &Deps{
		Workspace: "my-workspace",
		Project:   Project{Name: "my-project"},
	}
	if got := deps.GetWorkspaceName(); got != "my-workspace" {
		t.Errorf("GetWorkspaceName() = %q, want %q", got, "my-workspace")
	}
}

func TestGetWorkspaceName_FallsBackToProjectName(t *testing.T) {
	deps := &Deps{
		Project: Project{Name: "my-project"},
	}
	if got := deps.GetWorkspaceName(); got != "my-project" {
		t.Errorf("GetWorkspaceName() = %q, want %q", got, "my-project")
	}
}

func TestHasExplicitWorkspace(t *testing.T) {
	tests := []struct {
		name      string
		workspace string
		want      bool
	}{
		{"empty", "", false},
		{"set", "ws", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Deps{Workspace: tt.workspace}
			if got := d.HasExplicitWorkspace(); got != tt.want {
				t.Errorf("HasExplicitWorkspace() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLoadDepsLegacy(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := map[string]interface{}{
		"project":  map[string]interface{}{"name": "legacy-project"},
		"services": map[string]interface{}{},
	}
	data, _ := json.Marshal(cfg)
	path := filepath.Join(tmpDir, ".raioz.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	deps, err := LoadDepsLegacy(path)
	if err != nil {
		t.Fatalf("LoadDepsLegacy() error = %v", err)
	}
	if deps.Project.Name != "legacy-project" {
		t.Errorf("project name = %q, want %q", deps.Project.Name, "legacy-project")
	}
}

func TestLoadDepsLegacy_FileNotFound(t *testing.T) {
	_, err := LoadDepsLegacy("/nonexistent/file.json")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestFilterByProfiles_Empty(t *testing.T) {
	deps := &Deps{
		Services: map[string]Service{
			"api": {Profiles: []string{"dev"}},
			"web": {},
		},
		Infra: map[string]InfraEntry{
			"pg": {Inline: &Infra{Image: "postgres:16"}},
		},
	}

	result := FilterByProfiles(deps, nil)
	if result != deps {
		t.Error("FilterByProfiles with empty profiles should return original deps")
	}
}

func TestFilterByProfiles_FiltersServices(t *testing.T) {
	enabled := true
	deps := &Deps{
		Services: map[string]Service{
			"api":    {Profiles: []string{"dev"}, Enabled: &enabled},
			"worker": {Profiles: []string{"prod"}},
			"shared": {},
		},
		Infra: map[string]InfraEntry{
			"pg":    {Inline: &Infra{Image: "postgres:16", Profiles: []string{"dev"}}},
			"redis": {Inline: &Infra{Image: "redis:7"}},
		},
	}

	result := FilterByProfiles(deps, []string{"dev"})

	if _, ok := result.Services["api"]; !ok {
		t.Error("api should be included (profile=dev)")
	}
	if _, ok := result.Services["worker"]; ok {
		t.Error("worker should be excluded (profile=prod)")
	}
	if _, ok := result.Services["shared"]; !ok {
		t.Error("shared should be included (no profiles)")
	}
	if _, ok := result.Infra["pg"]; !ok {
		t.Error("pg should be included (profile=dev)")
	}
	if _, ok := result.Infra["redis"]; !ok {
		t.Error("redis should be included (no profiles)")
	}
}

func TestFilterByProfiles_ExcludesDisabled(t *testing.T) {
	disabled := false
	deps := &Deps{
		Services: map[string]Service{
			"api": {Profiles: []string{"dev"}, Enabled: &disabled},
		},
		Infra: map[string]InfraEntry{},
	}

	result := FilterByProfiles(deps, []string{"dev"})
	if _, ok := result.Services["api"]; ok {
		t.Error("disabled service should be excluded even if profile matches")
	}
}

func TestFilterByProfiles_MultipleProfiles(t *testing.T) {
	deps := &Deps{
		Services: map[string]Service{
			"api":    {Profiles: []string{"dev", "staging"}},
			"worker": {Profiles: []string{"prod"}},
		},
		Infra: map[string]InfraEntry{},
	}

	result := FilterByProfiles(deps, []string{"staging", "prod"})
	if _, ok := result.Services["api"]; !ok {
		t.Error("api should be included (staging matches)")
	}
	if _, ok := result.Services["worker"]; !ok {
		t.Error("worker should be included (prod matches)")
	}
}

// --- EnvValue tests ---

func TestEnvValue_UnmarshalJSON_Array(t *testing.T) {
	var ev EnvValue
	data := []byte(`["file1", "file2"]`)
	if err := json.Unmarshal(data, &ev); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if ev.IsObject {
		t.Error("expected IsObject=false for array")
	}
	if len(ev.Files) != 2 {
		t.Errorf("expected 2 files, got %d", len(ev.Files))
	}
	if ev.Files[0] != "file1" || ev.Files[1] != "file2" {
		t.Errorf("files = %v, want [file1 file2]", ev.Files)
	}
}

func TestEnvValue_UnmarshalJSON_Object(t *testing.T) {
	var ev EnvValue
	data := []byte(`{"DB_URL":"postgres://localhost","PORT":"5432"}`)
	if err := json.Unmarshal(data, &ev); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !ev.IsObject {
		t.Error("expected IsObject=true for object")
	}
	if ev.Variables["DB_URL"] != "postgres://localhost" {
		t.Errorf("DB_URL = %q", ev.Variables["DB_URL"])
	}
}

func TestEnvValue_UnmarshalJSON_Invalid(t *testing.T) {
	var ev EnvValue
	data := []byte(`12345`)
	if err := json.Unmarshal(data, &ev); err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestEnvValue_MarshalJSON_Object(t *testing.T) {
	ev := EnvValue{
		IsObject:  true,
		Variables: map[string]string{"K": "V"},
	}
	data, err := json.Marshal(ev)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]string
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("roundtrip: %v", err)
	}
	if m["K"] != "V" {
		t.Errorf("roundtrip K = %q", m["K"])
	}
}

func TestEnvValue_MarshalJSON_Array(t *testing.T) {
	ev := EnvValue{Files: []string{"a", "b"}}
	data, err := json.Marshal(ev)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var s []string
	if err := json.Unmarshal(data, &s); err != nil {
		t.Fatalf("roundtrip: %v", err)
	}
	if len(s) != 2 || s[0] != "a" || s[1] != "b" {
		t.Errorf("roundtrip = %v", s)
	}
}

func TestEnvValue_GetFilePaths(t *testing.T) {
	// File-based
	ev := &EnvValue{Files: []string{"a", "b"}}
	paths := ev.GetFilePaths()
	if len(paths) != 2 {
		t.Errorf("expected 2 paths, got %d", len(paths))
	}

	// Object-based returns nil
	ev2 := &EnvValue{IsObject: true, Variables: map[string]string{"K": "V"}}
	if ev2.GetFilePaths() != nil {
		t.Error("expected nil for object-based env")
	}
}

func TestEnvValue_GetVariables(t *testing.T) {
	// Object-based
	ev := &EnvValue{IsObject: true, Variables: map[string]string{"K": "V"}}
	vars := ev.GetVariables()
	if vars["K"] != "V" {
		t.Errorf("expected K=V, got %v", vars)
	}

	// File-based returns nil
	ev2 := &EnvValue{Files: []string{"a"}}
	if ev2.GetVariables() != nil {
		t.Error("expected nil for file-based env")
	}
}

// --- NetworkConfig tests ---

func TestNetworkConfig_GetSubnet(t *testing.T) {
	n := &NetworkConfig{Name: "net", Subnet: "10.0.0.0/16"}
	if got := n.GetSubnet(); got != "10.0.0.0/16" {
		t.Errorf("GetSubnet() = %q, want %q", got, "10.0.0.0/16")
	}
}

func TestNetworkConfig_HasSubnet(t *testing.T) {
	tests := []struct {
		name   string
		subnet string
		want   bool
	}{
		{"with subnet", "10.0.0.0/16", true},
		{"without subnet", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := &NetworkConfig{Subnet: tt.subnet}
			if got := n.HasSubnet(); got != tt.want {
				t.Errorf("HasSubnet() = %v, want %v", got, tt.want)
			}
		})
	}
}

// --- InfraEntry marshal/unmarshal ---

func TestInfraEntry_MarshalJSON_Path(t *testing.T) {
	e := InfraEntry{Path: "path/to/compose.yml"}
	data, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		t.Fatalf("roundtrip: %v", err)
	}
	if s != "path/to/compose.yml" {
		t.Errorf("got %q", s)
	}
}

func TestInfraEntry_MarshalJSON_Inline(t *testing.T) {
	e := InfraEntry{Inline: &Infra{Image: "postgres:16"}}
	data, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("roundtrip: %v", err)
	}
	if m["image"] != "postgres:16" {
		t.Errorf("image = %v", m["image"])
	}
}

func TestInfraEntry_MarshalJSON_Empty(t *testing.T) {
	e := InfraEntry{}
	data, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(data) != "null" {
		t.Errorf("expected null, got %s", data)
	}
}

func TestLoadDeps_WithWorkspace(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := map[string]interface{}{
		"workspace": "acme-corp",
		"project":   map[string]interface{}{"name": "api"},
		"services":  map[string]interface{}{},
	}
	data, _ := json.Marshal(cfg)
	path := filepath.Join(tmpDir, ".raioz.json")
	os.WriteFile(path, data, 0o644)

	deps, _, err := LoadDeps(path)
	if err != nil {
		t.Fatalf("LoadDeps: %v", err)
	}
	if deps.Workspace != "acme-corp" {
		t.Errorf("workspace = %q, want %q", deps.Workspace, "acme-corp")
	}
	if deps.GetWorkspaceName() != "acme-corp" {
		t.Errorf("GetWorkspaceName = %q, want %q", deps.GetWorkspaceName(), "acme-corp")
	}
	if !deps.HasExplicitWorkspace() {
		t.Error("HasExplicitWorkspace should be true")
	}
}

func TestLoadDeps_LegacyNetworkInProject(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := `{
		"project": {
			"name": "test",
			"network": "legacy-net"
		},
		"services": {}
	}`
	path := filepath.Join(tmpDir, ".raioz.json")
	os.WriteFile(path, []byte(cfg), 0o644)

	deps, _, err := LoadDeps(path)
	if err != nil {
		t.Fatalf("LoadDeps: %v", err)
	}
	if deps.Network.Name != "legacy-net" {
		t.Errorf("network name = %q, want %q", deps.Network.Name, "legacy-net")
	}
}

func TestLoadDeps_RootNetworkTakesPrecedence(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := `{
		"project": {
			"name": "test",
			"network": "legacy-net"
		},
		"network": "root-net",
		"services": {}
	}`
	path := filepath.Join(tmpDir, ".raioz.json")
	os.WriteFile(path, []byte(cfg), 0o644)

	deps, _, err := LoadDeps(path)
	if err != nil {
		t.Fatalf("LoadDeps: %v", err)
	}
	if deps.Network.Name != "root-net" {
		t.Errorf("network name = %q, want %q (root should take precedence)", deps.Network.Name, "root-net")
	}
}
