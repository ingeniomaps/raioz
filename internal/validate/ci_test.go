package validate

import (
	"testing"

	"raioz/internal/config"
)

// validDeps returns a minimally valid Deps for CI/validation tests.
func validDeps() *config.Deps {
	return &config.Deps{
		SchemaVersion: "1.0",
		Network:       config.NetworkConfig{Name: "test-network"},
		Project: config.Project{
			Name: "test-project",
		},
		Services: map[string]config.Service{
			"web": {
				Source: config.SourceConfig{
					Kind:  "image",
					Image: "nginx",
					Tag:   "latest",
				},
				Docker: &config.DockerConfig{
					Mode:      "dev",
					Ports:     []string{},
					Volumes:   []string{},
					DependsOn: []string{},
				},
			},
		},
		Infra: map[string]config.InfraEntry{},
		Env:   config.EnvConfig{Files: []string{}},
	}
}

func TestCIWrappers_ValidateSchema(t *testing.T) {
	// valid deps should pass
	if err := ValidateSchema(validDeps()); err != nil {
		t.Errorf("ValidateSchema(valid) unexpected error: %v", err)
	}

	// invalid: missing SchemaVersion fails
	bad := validDeps()
	bad.SchemaVersion = ""
	if err := ValidateSchema(bad); err == nil {
		t.Error("ValidateSchema(empty schemaVersion) expected error, got nil")
	}
}

func TestCIWrappers_ValidateProject(t *testing.T) {
	if err := ValidateProject(validDeps()); err != nil {
		t.Errorf("ValidateProject(valid) unexpected error: %v", err)
	}

	bad := validDeps()
	bad.Project.Name = ""
	if err := ValidateProject(bad); err == nil {
		t.Error("ValidateProject(no name) expected error, got nil")
	}

	bad2 := validDeps()
	bad2.Network.Name = ""
	if err := ValidateProject(bad2); err == nil {
		t.Error("ValidateProject(no network) expected error, got nil")
	}
}

func TestCIWrappers_ValidateServices(t *testing.T) {
	if err := ValidateServices(validDeps()); err != nil {
		t.Errorf("ValidateServices(valid) unexpected error: %v", err)
	}

	bad := validDeps()
	bad.Services = map[string]config.Service{
		"bad": {
			Source: config.SourceConfig{Kind: "unknown"},
		},
	}
	if err := ValidateServices(bad); err == nil {
		t.Error("ValidateServices(unknown kind) expected error, got nil")
	}
}

func TestCIWrappers_ValidateInfra(t *testing.T) {
	if err := ValidateInfra(validDeps()); err != nil {
		t.Errorf("ValidateInfra(valid) unexpected error: %v", err)
	}

	// infra with empty image
	bad := validDeps()
	bad.Infra = map[string]config.InfraEntry{
		"db": {Inline: &config.Infra{Image: ""}},
	}
	if err := ValidateInfra(bad); err == nil {
		t.Error("ValidateInfra(empty image) expected error, got nil")
	}

	// valid inline infra
	good := validDeps()
	good.Infra = map[string]config.InfraEntry{
		"db": {Inline: &config.Infra{Image: "postgres", Tag: "15"}},
	}
	if err := ValidateInfra(good); err != nil {
		t.Errorf("ValidateInfra(valid infra) unexpected error: %v", err)
	}
}

func TestCIWrappers_ValidateDependencies(t *testing.T) {
	if err := ValidateDependencies(validDeps()); err != nil {
		t.Errorf("ValidateDependencies(valid) unexpected error: %v", err)
	}

	// service depends on missing
	bad := validDeps()
	bad.Services = map[string]config.Service{
		"web": {
			Source: config.SourceConfig{Kind: "image", Image: "nginx", Tag: "latest"},
			Docker: &config.DockerConfig{Mode: "dev", DependsOn: []string{"missing"}},
		},
	}
	if err := ValidateDependencies(bad); err == nil {
		t.Error("ValidateDependencies(missing dep) expected error, got nil")
	}
}

func TestCIWrappers_DockerChecks_NoPanic(t *testing.T) {
	// These may fail in environments without Docker, but must not panic.
	_ = CheckDockerInstalled()
	_ = CheckDockerRunning()
}
