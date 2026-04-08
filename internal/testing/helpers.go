package testing

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"raioz/internal/config"
)

// CreateTestDepsJSON creates a temporary .raioz.json file for testing
func CreateTestDepsJSON(tdir string, deps *config.Deps) (string, error) {
	depsPath := filepath.Join(tdir, ".raioz.json")
	data, err := json.MarshalIndent(deps, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal deps: %w", err)
	}

	if err := os.WriteFile(depsPath, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write .raioz.json: %w", err)
	}

	return depsPath, nil
}

// CreateMinimalTestDeps creates a minimal valid Deps configuration for testing
func CreateMinimalTestDeps() *config.Deps {
	return &config.Deps{
		SchemaVersion: "1.0",
		Network:       config.NetworkConfig{Name: "test-network", IsObject: false},
		Project: config.Project{
			Name: "test-project",
		},
		Services: map[string]config.Service{},
		Infra:    map[string]config.InfraEntry{},
		Env: config.EnvConfig{
			UseGlobal: true,
			Files:     []string{},
		},
	}
}

// CreateTestDepsWithService creates a test Deps with one service
func CreateTestDepsWithService(serviceName string, sourceKind string) *config.Deps {
	deps := CreateMinimalTestDeps()

	var sourceConfig config.SourceConfig
	var dockerConfig config.DockerConfig

	if sourceKind == "git" {
		sourceConfig = config.SourceConfig{
			Kind:   "git",
			Repo:   "git@github.com:test/repo.git",
			Branch: "main",
			Path:   fmt.Sprintf("services/%s", serviceName),
		}
	} else if sourceKind == "image" {
		sourceConfig = config.SourceConfig{
			Kind:  "image",
			Image: "test/image",
			Tag:   "latest",
		}
	} else {
		sourceConfig = config.SourceConfig{
			Kind: sourceKind,
		}
	}

	dockerConfig = config.DockerConfig{
		Mode:  "dev",
		Ports: []string{"3000:3000"},
	}

	deps.Services[serviceName] = config.Service{
		Source: sourceConfig,
		Docker: &dockerConfig,
	}

	return deps
}

// CreateTestDepsWithInfra creates a test Deps with one infra service
func CreateTestDepsWithInfra(infraName string) *config.Deps {
	deps := CreateMinimalTestDeps()
	deps.Infra[infraName] = config.InfraEntry{Inline: &config.Infra{
		Image: "postgres",
		Tag:   "15",
		Ports: []string{"5432:5432"},
	}}
	return deps
}

// CreateInvalidDepsJSON creates an invalid .raioz.json for testing
func CreateInvalidDepsJSON(tdir string) (string, error) {
	depsPath := filepath.Join(tdir, ".raioz.json")
	invalidJSON := `{
		"schemaVersion": "1.0",
		"project": {
			"invalid": "field"
		}
	}`
	if err := os.WriteFile(depsPath, []byte(invalidJSON), 0644); err != nil {
		return "", fmt.Errorf("failed to write invalid .raioz.json: %w", err)
	}
	return depsPath, nil
}

// CreateMalformedDepsJSON creates a malformed JSON file
func CreateMalformedDepsJSON(tdir string) (string, error) {
	depsPath := filepath.Join(tdir, ".raioz.json")
	malformedJSON := `{
		"schemaVersion": "1.0",
		"project": {
			"name": "test",
		}
	}`
	if err := os.WriteFile(depsPath, []byte(malformedJSON), 0644); err != nil {
		return "", fmt.Errorf("failed to write malformed .raioz.json: %w", err)
	}
	return depsPath, nil
}
