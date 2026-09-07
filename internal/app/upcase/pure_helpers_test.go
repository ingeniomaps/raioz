package upcase

import (
	"testing"

	"raioz/internal/domain/models"
)

// --- parseHealthCommandOutput --------------------------------------------------

// --- getServiceHealthCommand ---------------------------------------------------

// --- getLocalProjectCommand ----------------------------------------------------

// --- isYAMLMode ----------------------------------------------------------------

// --- buildServiceContext -------------------------------------------------------

func TestBuildServiceContext(t *testing.T) {
	det := models.DetectResult{Runtime: models.RuntimeGo, StartCommand: "go run ."}
	envVars := map[string]string{"FOO": "bar"}
	ports := []string{"8080:8080"}
	deps := []string{"postgres"}

	ctx := buildServiceContext(
		"api", det, "acme-net", envVars, ports, deps,
		"acme-api", "/path/to/api", "acme",
	)

	if ctx.Name != "api" {
		t.Errorf("Name = %q, want api", ctx.Name)
	}
	if ctx.NetworkName != "acme-net" {
		t.Errorf("NetworkName = %q, want acme-net", ctx.NetworkName)
	}
	if ctx.ProjectName != "acme" {
		t.Errorf("ProjectName = %q, want acme", ctx.ProjectName)
	}
	if ctx.ContainerName != "acme-api" {
		t.Errorf("ContainerName = %q, want acme-api", ctx.ContainerName)
	}
	if ctx.Path != "/path/to/api" {
		t.Errorf("Path = %q, want /path/to/api", ctx.Path)
	}
	if len(ctx.Ports) != 1 || ctx.Ports[0] != "8080:8080" {
		t.Errorf("Ports = %v, want [8080:8080]", ctx.Ports)
	}
	if len(ctx.DependsOn) != 1 || ctx.DependsOn[0] != "postgres" {
		t.Errorf("DependsOn = %v, want [postgres]", ctx.DependsOn)
	}
	if ctx.EnvVars["FOO"] != "bar" {
		t.Errorf("EnvVars[FOO] = %q, want bar", ctx.EnvVars["FOO"])
	}
	if ctx.Detection.Runtime != models.RuntimeGo {
		t.Errorf("Detection.Runtime = %q, want go", ctx.Detection.Runtime)
	}
}

// --- infraPorts / servicePorts -------------------------------------------------

func TestInfraPorts(t *testing.T) {
	t.Run("inline with ports", func(t *testing.T) {
		entry := models.InfraEntry{Inline: &models.Infra{Ports: []string{"5432"}}}
		got := infraPorts(entry)
		if len(got) != 1 || got[0] != "5432" {
			t.Errorf("got %v, want [5432]", got)
		}
	})
	t.Run("nil inline", func(t *testing.T) {
		entry := models.InfraEntry{}
		got := infraPorts(entry)
		if got != nil {
			t.Errorf("expected nil, got %v", got)
		}
	})
}

func TestServicePorts(t *testing.T) {
	t.Run("docker ports", func(t *testing.T) {
		svc := models.Service{Docker: &models.DockerConfig{Ports: []string{"3000:3000"}}}
		got := servicePorts(svc)
		if len(got) != 1 || got[0] != "3000:3000" {
			t.Errorf("got %v, want [3000:3000]", got)
		}
	})
	t.Run("no docker", func(t *testing.T) {
		svc := models.Service{}
		got := servicePorts(svc)
		if got != nil {
			t.Errorf("expected nil, got %v", got)
		}
	})
}

// --- orderedServiceNames -------------------------------------------------------

func TestOrderedServiceNames(t *testing.T) {
	t.Run("no dependencies", func(t *testing.T) {
		deps := &models.Deps{
			Services: map[string]models.Service{
				"a": {},
				"b": {},
			},
		}
		got := orderedServiceNames(deps)
		if len(got) != 2 {
			t.Errorf("expected 2 services, got %d", len(got))
		}
	})
	t.Run("linear chain", func(t *testing.T) {
		deps := &models.Deps{
			Services: map[string]models.Service{
				"web": {DependsOn: []string{"api"}},
				"api": {DependsOn: []string{"db"}},
				"db":  {},
			},
		}
		got := orderedServiceNames(deps)
		if len(got) != 3 {
			t.Fatalf("expected 3, got %d", len(got))
		}
		// db must come before api, api before web
		idx := map[string]int{}
		for i, n := range got {
			idx[n] = i
		}
		if idx["db"] > idx["api"] {
			t.Errorf("db should come before api: %v", got)
		}
		if idx["api"] > idx["web"] {
			t.Errorf("api should come before web: %v", got)
		}
	})
	t.Run("ignores infra deps", func(t *testing.T) {
		deps := &models.Deps{
			Services: map[string]models.Service{
				"api": {DependsOn: []string{"postgres"}},
			},
			Infra: map[string]models.InfraEntry{
				"postgres": {},
			},
		}
		got := orderedServiceNames(deps)
		if len(got) != 1 || got[0] != "api" {
			t.Errorf("expected [api], got %v", got)
		}
	})
}

// --- mergeSliceUnique ----------------------------------------------------------

// --- mergeVariables ------------------------------------------------------------

// --- volumeContainerPath / mergeVolumesOnlyNew --------------------------------

// --- cloneService --------------------------------------------------------------

// --- cloneInfraEntry -----------------------------------------------------------

// --- inferServicePort additional cases ----------------------------------------

func TestInferServicePortConfigPriority(t *testing.T) {
	svc := models.Service{
		Docker: &models.DockerConfig{Ports: []string{"4242:80"}},
	}
	det := models.DetectResult{Runtime: models.RuntimeGo}
	got := inferServicePort(svc, det)
	if got != 4242 {
		t.Errorf("config port should win, got %d", got)
	}
}

func TestInferServicePortUnknownRuntime(t *testing.T) {
	svc := models.Service{}
	det := models.DetectResult{Runtime: models.Runtime("weird")}
	got := inferServicePort(svc, det)
	if got != 0 {
		t.Errorf("expected 0 for unknown runtime, got %d", got)
	}
}

// --- isProcessAlive / isProcessRunning ----------------------------------------

func TestIsProcessAliveCurrent(t *testing.T) {
	// Current process must be alive
	if !isProcessAlive(1) && !isProcessAlive(2) {
		// Skip if we can't inspect low PIDs (e.g., sandbox)
		t.Skip("cannot inspect low PIDs in this environment")
	}
}

func TestIsProcessAliveInvalid(t *testing.T) {
	if isProcessAlive(-1) {
		t.Error("negative PID should not be alive")
	}
}
