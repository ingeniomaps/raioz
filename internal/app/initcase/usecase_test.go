package initcase

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"raioz/internal/config"
)

func TestExecuteHappyPathNoOptionals(t *testing.T) {
	initI18n(t)

	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, ".raioz.json")

	// project=test-app, network=default, no service, no infra
	uc, out := newTestUseCaseWithIO("test-app\n\nn\nn\n")

	err := uc.Execute(context.Background(), Options{OutputPath: outputPath})
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}

	data, _ := os.ReadFile(outputPath)
	var deps config.Deps
	json.Unmarshal(data, &deps)

	if deps.Project.Name != "test-app" {
		t.Errorf("Project.Name = %s, want test-app", deps.Project.Name)
	}
	if len(deps.Services) != 0 {
		t.Errorf("Services should be empty, got %d", len(deps.Services))
	}

	output := out.String()
	if !strings.Contains(output, "Welcome") {
		t.Error("should show welcome message")
	}
}

func TestExecuteWithService(t *testing.T) {
	initI18n(t)

	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, ".raioz.json")

	// project, network, add service=yes, name, kind=git, repo, branch, path, port, mode, another=no, no infra
	input := "my-app\n\ny\napi\ngit\ngit@github.com:org/api.git\nmain\nservices/api\n8080:8080\ndev\nn\nn\n"
	uc, _ := newTestUseCaseWithIO(input)

	err := uc.Execute(context.Background(), Options{OutputPath: outputPath})
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}

	data, _ := os.ReadFile(outputPath)
	var deps config.Deps
	json.Unmarshal(data, &deps)

	if len(deps.Services) != 1 {
		t.Fatalf("Services count = %d, want 1", len(deps.Services))
	}
	svc, ok := deps.Services["api"]
	if !ok {
		t.Fatal("service 'api' not found")
	}
	if svc.Source.Kind != "git" {
		t.Errorf("Source.Kind = %s, want git", svc.Source.Kind)
	}
	if svc.Source.Repo != "git@github.com:org/api.git" {
		t.Errorf("Source.Repo = %s, want git@github.com:org/api.git", svc.Source.Repo)
	}
	if svc.Docker == nil || svc.Docker.Ports[0] != "8080:8080" {
		t.Error("Docker port not set correctly")
	}
}

func TestExecuteWithMultipleServices(t *testing.T) {
	initI18n(t)

	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, ".raioz.json")

	// add service=yes, first service (git), add another=yes, second service (image), add another=no, no infra
	input := "my-app\n\ny\napi\ngit\ngit@github.com:org/api.git\nmain\nservices/api\n3000:3000\ndev\ny\nweb\nimage\nnginx\nlatest\n80:80\nprod\nn\nn\n"
	uc, _ := newTestUseCaseWithIO(input)

	err := uc.Execute(context.Background(), Options{OutputPath: outputPath})
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}

	data, _ := os.ReadFile(outputPath)
	var deps config.Deps
	json.Unmarshal(data, &deps)

	if len(deps.Services) != 2 {
		t.Fatalf("Services count = %d, want 2", len(deps.Services))
	}
	if _, ok := deps.Services["api"]; !ok {
		t.Error("service 'api' not found")
	}
	if _, ok := deps.Services["web"]; !ok {
		t.Error("service 'web' not found")
	}
}

func TestExecuteWithImageService(t *testing.T) {
	initI18n(t)

	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, ".raioz.json")

	// service kind=image, another=no, no infra
	input := "my-app\n\ny\nfrontend\nimage\nnginx\nlatest\n80:80\nprod\nn\nn\n"
	uc, _ := newTestUseCaseWithIO(input)

	err := uc.Execute(context.Background(), Options{OutputPath: outputPath})
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}

	data, _ := os.ReadFile(outputPath)
	var deps config.Deps
	json.Unmarshal(data, &deps)

	svc := deps.Services["frontend"]
	if svc.Source.Kind != "image" {
		t.Errorf("Source.Kind = %s, want image", svc.Source.Kind)
	}
	if svc.Source.Image != "nginx" {
		t.Errorf("Source.Image = %s, want nginx", svc.Source.Image)
	}
}

func TestExecuteWithInfraPresets(t *testing.T) {
	initI18n(t)

	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, ".raioz.json")

	// no service, infra=yes, select 1 (postgres) and 2 (redis)
	input := "my-app\n\nn\ny\n1,2\n"
	uc, _ := newTestUseCaseWithIO(input)

	err := uc.Execute(context.Background(), Options{OutputPath: outputPath})
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}

	data, _ := os.ReadFile(outputPath)
	var deps config.Deps
	json.Unmarshal(data, &deps)

	if len(deps.Infra) != 2 {
		t.Fatalf("Infra count = %d, want 2", len(deps.Infra))
	}
}

func TestExecuteWithCustomInfra(t *testing.T) {
	initI18n(t)

	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, ".raioz.json")

	// no service, infra=yes, select 5 (custom), name, image, tag, port
	input := "my-app\n\nn\ny\n5\nelastic\nelasticsearch\n8.12\n9200:9200\n"
	uc, _ := newTestUseCaseWithIO(input)

	err := uc.Execute(context.Background(), Options{OutputPath: outputPath})
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}

	data, _ := os.ReadFile(outputPath)
	var deps config.Deps
	json.Unmarshal(data, &deps)

	entry, ok := deps.Infra["elastic"]
	if !ok {
		t.Fatal("infra 'elastic' not found")
	}
	if entry.Inline == nil || entry.Inline.Image != "elasticsearch" {
		t.Error("custom infra image not set correctly")
	}
}

func TestExecuteWithDefaults(t *testing.T) {
	initI18n(t)

	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, ".raioz.json")

	// All defaults, no optionals
	uc, _ := newTestUseCaseWithIO("\n\nn\nn\n")

	err := uc.Execute(context.Background(), Options{OutputPath: outputPath})
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}

	data, _ := os.ReadFile(outputPath)
	var deps config.Deps
	json.Unmarshal(data, &deps)

	if deps.Project.Name != "my-project" {
		t.Errorf("Project.Name = %s, want my-project", deps.Project.Name)
	}
	if deps.Env.Files[1] != "projects/my-project" {
		t.Errorf("Env.Files[1] = %s, want projects/my-project", deps.Env.Files[1])
	}
}

func TestExecuteFileExistsUserCancels(t *testing.T) {
	initI18n(t)

	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, ".raioz.json")
	os.WriteFile(outputPath, []byte(`{"old": true}`), 0644)

	uc, _ := newTestUseCaseWithIO("n\n")

	err := uc.Execute(context.Background(), Options{OutputPath: outputPath})
	if err != nil {
		t.Fatalf("should not error on cancel: %v", err)
	}

	data, _ := os.ReadFile(outputPath)
	if !strings.Contains(string(data), `"old"`) {
		t.Error("original file should not be overwritten")
	}
}

func TestExecuteFileExistsUserConfirms(t *testing.T) {
	initI18n(t)

	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, ".raioz.json")
	os.WriteFile(outputPath, []byte(`{"old": true}`), 0644)

	uc, _ := newTestUseCaseWithIO("y\nnew-project\n\nn\nn\n")

	err := uc.Execute(context.Background(), Options{OutputPath: outputPath})
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}

	data, _ := os.ReadFile(outputPath)
	var deps config.Deps
	json.Unmarshal(data, &deps)

	if deps.Project.Name != "new-project" {
		t.Errorf("Project.Name = %s, want new-project", deps.Project.Name)
	}
}

func TestExecuteInvalidOutputPath(t *testing.T) {
	initI18n(t)

	uc, _ := newTestUseCaseWithIO("test\n\nn\nn\n")

	err := uc.Execute(context.Background(), Options{
		OutputPath: "/nonexistent/deeply/nested/dir/config.json",
	})

	if err == nil {
		t.Error("expected error for invalid output path")
	}
}
