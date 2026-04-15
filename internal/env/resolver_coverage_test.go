package env

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"raioz/internal/config"
	"raioz/internal/workspace"
)

func testWS(t *testing.T) *workspace.Workspace {
	t.Helper()
	dir := t.TempDir()
	ws := &workspace.Workspace{
		Root:        filepath.Join(dir, "ws"),
		ServicesDir: filepath.Join(dir, "services"),
		EnvDir:      filepath.Join(dir, "env"),
	}
	if err := EnsureEnvDirs(ws); err != nil {
		t.Fatalf("EnsureEnvDirs: %v", err)
	}
	return ws
}

func TestResolveProjectEnv_NilProjectEnv(t *testing.T) {
	ws := testWS(t)
	deps := &config.Deps{Project: config.Project{Name: "proj"}}
	result, err := ResolveProjectEnv(ws, deps, t.TempDir())
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if result != "" {
		t.Errorf("expected empty, got %q", result)
	}
}

func TestResolveProjectEnv_ObjectVariables(t *testing.T) {
	ws := testWS(t)
	deps := &config.Deps{
		Project: config.Project{
			Name: "proj",
			Env: &config.EnvValue{
				IsObject:  true,
				Variables: map[string]string{"DB": "postgres://localhost", "PORT": "5432"},
			},
		},
	}

	result, err := ResolveProjectEnv(ws, deps, t.TempDir())
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if result == "" {
		t.Fatal("expected non-empty path")
	}

	data, err := os.ReadFile(result)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "DB=") {
		t.Error("expected DB in output")
	}
	if !strings.Contains(content, "PORT=5432") {
		t.Error("expected PORT=5432 in output")
	}
}

func TestResolveProjectEnv_ObjectWithSpacesInValues(t *testing.T) {
	ws := testWS(t)
	deps := &config.Deps{
		Project: config.Project{
			Name: "proj",
			Env: &config.EnvValue{
				IsObject:  true,
				Variables: map[string]string{"DESC": "hello world"},
			},
		},
	}

	result, err := ResolveProjectEnv(ws, deps, t.TempDir())
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	data, _ := os.ReadFile(result)
	// Value with space should be quoted
	if !strings.Contains(string(data), `"hello world"`) {
		t.Errorf("expected quoted value, got %s", data)
	}
}

func TestResolveProjectEnv_DotArray(t *testing.T) {
	projectDir := t.TempDir()
	ws := testWS(t)

	// Create .env in project dir
	envPath := filepath.Join(projectDir, ".env")
	os.WriteFile(envPath, []byte("KEY=val\n"), 0o644)

	deps := &config.Deps{
		Project: config.Project{
			Name: "proj",
			Env:  &config.EnvValue{Files: []string{"."}},
		},
	}

	result, err := ResolveProjectEnv(ws, deps, projectDir)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if result != envPath {
		t.Errorf("got %q, want %q", result, envPath)
	}
}

func TestResolveProjectEnv_DotArrayNoEnvFile(t *testing.T) {
	projectDir := t.TempDir()
	ws := testWS(t)

	deps := &config.Deps{
		Project: config.Project{
			Name: "proj",
			Env:  &config.EnvValue{Files: []string{"."}},
		},
	}

	result, err := ResolveProjectEnv(ws, deps, projectDir)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if result != "" {
		t.Errorf("expected empty when .env does not exist, got %q", result)
	}
}

func TestResolveProjectEnv_ArrayWithNamedFile(t *testing.T) {
	ws := testWS(t)
	projectDir := t.TempDir()

	// Create the env file in the services dir
	svcEnvPath := filepath.Join(ws.EnvDir, "services", "myservice.env")
	os.WriteFile(svcEnvPath, []byte("SVC_KEY=svc_val\n"), 0o644)

	deps := &config.Deps{
		Project: config.Project{
			Name: "proj",
			Env:  &config.EnvValue{Files: []string{"services/myservice"}},
		},
	}

	result, err := ResolveProjectEnv(ws, deps, projectDir)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if result == "" {
		t.Error("expected non-empty path for named file")
	}
}

func TestResolveEnvFiles_IncludesProjectLevel(t *testing.T) {
	ws := testWS(t)

	// Create global env
	os.WriteFile(filepath.Join(ws.EnvDir, "global.env"), []byte("G=1\n"), 0o644)

	// Create a project env file
	projDir := filepath.Join(ws.EnvDir, "projects")
	os.WriteFile(filepath.Join(projDir, "myproj.env"), []byte("P=2\n"), 0o644)

	deps := &config.Deps{
		Project: config.Project{Name: "myproj"},
		Env: config.EnvConfig{
			UseGlobal: true,
			Files:     []string{"myproj"},
		},
	}

	paths, err := ResolveEnvFiles(ws, deps, "svc", nil, "/fake/project.env", true, "")
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	// Should include global + project env path + project file
	if len(paths) < 2 {
		t.Errorf("expected at least 2 paths, got %d: %v", len(paths), paths)
	}

	// project env path should be included
	found := false
	for _, p := range paths {
		if p == "/fake/project.env" {
			found = true
		}
	}
	if !found {
		t.Error("expected projectEnvPath to be included when includeProjectLevel=true")
	}
}

func TestResolveEnvFiles_SkipsServiceFilesInProjectContext(t *testing.T) {
	ws := testWS(t)

	deps := &config.Deps{
		Project: config.Project{Name: "proj"},
		Env: config.EnvConfig{
			UseGlobal: false,
			Files:     []string{"services/myservice"}, // should be skipped at project level
		},
	}

	paths, err := ResolveEnvFiles(ws, deps, "svc", nil, "", true, "")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	// "services/myservice" in env.Files should be skipped when includeProjectLevel=true
	if len(paths) != 0 {
		t.Errorf("expected 0 paths (service files skipped at project level), got %d", len(paths))
	}
}

func TestResolveServiceEnvFile_ProjectRelativePath(t *testing.T) {
	ws := testWS(t)
	projectDir := t.TempDir()

	// Create a project-relative env file
	envFile := filepath.Join(projectDir, ".env.shared")
	os.WriteFile(envFile, []byte("SHARED=1\n"), 0o644)

	deps := &config.Deps{
		Project: config.Project{Name: "proj"},
	}

	resolved, err := resolveServiceEnvFile(ws, deps, "api", ".env.shared", "", projectDir)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if resolved != envFile {
		t.Errorf("got %q, want %q", resolved, envFile)
	}
}

func TestResolveServiceEnvFile_DotReturnsProjectEnvPath(t *testing.T) {
	ws := testWS(t)
	deps := &config.Deps{
		Project: config.Project{Name: "proj"},
	}

	resolved, err := resolveServiceEnvFile(ws, deps, "api", ".", "/fake/project.env", "")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if resolved != "/fake/project.env" {
		t.Errorf("got %q, want %q", resolved, "/fake/project.env")
	}
}

func TestResolveServiceEnvFile_ProjectSpecificLocation(t *testing.T) {
	ws := testWS(t)

	// Create project-specific service env file
	projSvcDir := filepath.Join(ws.EnvDir, "projects", "proj", "services")
	os.MkdirAll(projSvcDir, 0o755)
	envFile := filepath.Join(projSvcDir, "api.env")
	os.WriteFile(envFile, []byte("API_KEY=123\n"), 0o644)

	deps := &config.Deps{
		Project: config.Project{Name: "proj"},
	}

	resolved, err := resolveServiceEnvFile(ws, deps, "api", "api", "", "")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if resolved != envFile {
		t.Errorf("got %q, want %q", resolved, envFile)
	}
}

func TestResolveServiceEnvFile_FallbackToShared(t *testing.T) {
	ws := testWS(t)

	// Create shared service env file (no project-specific one)
	sharedFile := filepath.Join(ws.EnvDir, "services", "api.env")
	os.WriteFile(sharedFile, []byte("SHARED_KEY=456\n"), 0o644)

	deps := &config.Deps{
		Project: config.Project{Name: "proj"},
	}

	resolved, err := resolveServiceEnvFile(ws, deps, "api", "api", "", "")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if resolved != sharedFile {
		t.Errorf("got %q, want %q", resolved, sharedFile)
	}
}

func TestResolveEnvFileForService_NilEnvValue(t *testing.T) {
	ws := testWS(t)
	deps := &config.Deps{Project: config.Project{Name: "proj"}}

	result, err := ResolveEnvFileForService(ws, deps, "api", nil, "", "")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if result != "" {
		t.Errorf("expected empty, got %q", result)
	}
}

func TestResolveEnvFileForService_ObjectEnv(t *testing.T) {
	ws := testWS(t)
	deps := &config.Deps{Project: config.Project{Name: "proj"}}
	servicePath := t.TempDir()

	envVal := &config.EnvValue{
		IsObject:  true,
		Variables: map[string]string{"FOO": "bar"},
	}

	result, err := ResolveEnvFileForService(ws, deps, "api", envVal, "", servicePath)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if result == "" {
		t.Fatal("expected non-empty path")
	}

	data, _ := os.ReadFile(result)
	if !strings.Contains(string(data), "FOO=bar") {
		t.Errorf("expected FOO=bar in file, got %s", data)
	}
}

func TestResolveEnvFileForService_DotEnvResolution(t *testing.T) {
	ws := testWS(t)
	projectDir := t.TempDir()

	// Create .env in project dir
	dotEnv := filepath.Join(projectDir, ".env")
	os.WriteFile(dotEnv, []byte("PROJECT_VAR=pval\n"), 0o644)

	// Also create a service env file
	svcEnv := filepath.Join(ws.EnvDir, "services", "api.env")
	os.WriteFile(svcEnv, []byte("SVC_VAR=sval\n"), 0o644)

	deps := &config.Deps{
		Project: config.Project{Name: "proj"},
		Env:     config.EnvConfig{UseGlobal: false},
	}

	envVal := &config.EnvValue{Files: []string{".", "api"}}

	result, err := ResolveEnvFileForService(ws, deps, "api", envVal, projectDir, "")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	// Should have created a combined file since there are multiple resolved paths
	if result == "" {
		t.Error("expected non-empty path")
	}
}

func TestProcessTemplate_DefaultSyntax(t *testing.T) {
	tpl := "DB=${DB_HOST:-localhost}:${DB_PORT:-5432}"
	vars := map[string]string{"DB_HOST": "myhost", "DB_PORT": "3306"}
	result := processTemplate(tpl, vars)

	if !strings.Contains(result, "myhost") {
		t.Errorf("expected myhost in result, got %q", result)
	}
}

func TestProcessTemplate_EmptyVars(t *testing.T) {
	tpl := "value=${KEY}"
	result := processTemplate(tpl, map[string]string{})
	// KEY not found, template stays as-is
	if !strings.Contains(result, "${KEY}") {
		t.Errorf("expected ${KEY} to remain, got %q", result)
	}
}

func TestProcessTemplate_MultipleReplacements(t *testing.T) {
	tpl := "$A and $B and ${C}"
	vars := map[string]string{"A": "1", "B": "2", "C": "3"}
	result := processTemplate(tpl, vars)
	if result != "1 and 2 and 3" {
		t.Errorf("got %q, want '1 and 2 and 3'", result)
	}
}

func TestGenerateEnvFromTemplate_SkipsWhenProjectEnvExists(t *testing.T) {
	ws := testWS(t)
	servicePath := t.TempDir()

	// Create template
	os.WriteFile(filepath.Join(servicePath, ".env.example"), []byte("X=default\n"), 0o644)

	deps := &config.Deps{Project: config.Project{Name: "proj"}}
	svc := config.Service{}

	// When serviceName == project name and projectEnvPath is set, should skip
	err := GenerateEnvFromTemplate(ws, deps, "proj", servicePath, svc, "/fake/project.env", t.TempDir())
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	// .env should NOT be created (skipped due to project env)
	if _, statErr := os.Stat(filepath.Join(servicePath, ".env")); statErr == nil {
		t.Error("expected .env to NOT be created when projectEnvPath is set for project service")
	}
}

func TestGenerateEnvFromTemplate_CreatesEnvFromTemplate(t *testing.T) {
	ws := testWS(t)
	servicePath := t.TempDir()
	projectDir := t.TempDir()

	// Create template
	os.WriteFile(filepath.Join(servicePath, ".env.template"), []byte("APP_NAME=myapp\nPORT=3000\n"), 0o644)

	deps := &config.Deps{Project: config.Project{Name: "proj"}}
	svc := config.Service{}

	err := GenerateEnvFromTemplate(ws, deps, "api", servicePath, svc, "", projectDir)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	// .env should now exist with the template content
	data, readErr := os.ReadFile(filepath.Join(servicePath, ".env"))
	if readErr != nil {
		t.Fatalf("expected .env to be created: %v", readErr)
	}
	parsed := parseEnvContent(string(data))
	if parsed["APP_NAME"] != "myapp" {
		t.Errorf("APP_NAME = %q, want %q", parsed["APP_NAME"], "myapp")
	}
}

func TestGenerateEnvFromTemplate_MergesWithExistingEnv(t *testing.T) {
	ws := testWS(t)
	servicePath := t.TempDir()
	projectDir := t.TempDir()

	// Create existing .env
	os.WriteFile(filepath.Join(servicePath, ".env"), []byte("EXISTING=keep\nPORT=8080\n"), 0o644)

	// Create template
	os.WriteFile(filepath.Join(servicePath, ".env.example"), []byte("PORT=3000\nNEW_VAR=hello\n"), 0o644)

	deps := &config.Deps{Project: config.Project{Name: "proj"}}
	svc := config.Service{}

	err := GenerateEnvFromTemplate(ws, deps, "api", servicePath, svc, "", projectDir)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(servicePath, ".env"))
	parsed := parseEnvContent(string(data))
	if parsed["EXISTING"] != "keep" {
		t.Errorf("EXISTING should be preserved: %q", parsed["EXISTING"])
	}
}

func TestGenerateEnvFromTemplate_WithServiceEnvVars(t *testing.T) {
	ws := testWS(t)
	servicePath := t.TempDir()
	projectDir := t.TempDir()

	// Template with placeholders
	os.WriteFile(filepath.Join(servicePath, ".env.example"), []byte("DB_URL=${DB_URL}\nPORT=3000\n"), 0o644)

	deps := &config.Deps{Project: config.Project{Name: "proj"}}
	svc := config.Service{
		Env: &config.EnvValue{
			IsObject:  true,
			Variables: map[string]string{"DB_URL": "postgres://mydb"},
		},
	}

	err := GenerateEnvFromTemplate(ws, deps, "api", servicePath, svc, "", projectDir)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(servicePath, ".env"))
	parsed := parseEnvContent(string(data))
	if parsed["DB_URL"] != "postgres://mydb" {
		t.Errorf("DB_URL = %q, want %q", parsed["DB_URL"], "postgres://mydb")
	}
}

func TestCreateCombinedEnvFile_WithDotEnv(t *testing.T) {
	ws := testWS(t)
	servicePath := t.TempDir()

	// Create source env files
	file1 := filepath.Join(t.TempDir(), "a.env")
	file2 := filepath.Join(t.TempDir(), "b.env")
	os.WriteFile(file1, []byte("A=1\n"), 0o644)
	os.WriteFile(file2, []byte("B=2\n"), 0o644)

	result, err := createCombinedEnvFile(ws, "api", []string{file1, file2}, true, servicePath, "")
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	// When hasDotEnv=true and servicePath is set, should write to servicePath/.env
	expected := filepath.Join(servicePath, ".env")
	if result != expected {
		t.Errorf("got %q, want %q", result, expected)
	}

	data, _ := os.ReadFile(result)
	parsed := parseEnvContent(string(data))
	if parsed["A"] != "1" || parsed["B"] != "2" {
		t.Errorf("expected A=1 and B=2, got %v", parsed)
	}
}

func TestCreateCombinedEnvFile_WithoutDotEnv(t *testing.T) {
	ws := testWS(t)
	os.MkdirAll(ws.Root, 0o755)

	file1 := filepath.Join(t.TempDir(), "a.env")
	os.WriteFile(file1, []byte("A=1\n"), 0o644)

	result, err := createCombinedEnvFile(ws, "api", []string{file1}, false, "", "")
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	// Should write to ws.Root/.env.api
	expected := filepath.Join(ws.Root, ".env.api")
	if result != expected {
		t.Errorf("got %q, want %q", result, expected)
	}
}
