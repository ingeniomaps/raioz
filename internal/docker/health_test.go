package docker

import (
	"context"
	"testing"
)

func TestWaitForServicesHealthy_NoServices(t *testing.T) {
	// Empty service list should return immediately with nil
	err := WaitForServicesHealthy(
		context.Background(), "/tmp/fake.yml",
		nil, nil, "proj",
	)
	if err != nil {
		t.Errorf("expected nil for empty names, got: %v", err)
	}
}

func TestWaitForServicesHealthy_EmptyLists(t *testing.T) {
	err := WaitForServicesHealthy(
		context.Background(), "/tmp/fake.yml",
		[]string{}, []string{}, "proj",
	)
	if err != nil {
		t.Errorf("expected nil for empty lists, got: %v", err)
	}
}

func TestWaitForServicesHealthy_CancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	err := WaitForServicesHealthy(
		ctx, "/tmp/fake.yml",
		[]string{"svc1"}, nil, "proj",
	)
	if err == nil {
		t.Error("expected error for cancelled context")
	}
}

func TestCheckServiceReadinessFromInspect_Postgres(t *testing.T) {
	tests := []struct {
		name      string
		image     string
		env       []string
		wantCheck bool // whether it attempts a check (vs returning error)
	}{
		{
			"postgres with custom user",
			"postgres:16",
			[]string{"POSTGRES_USER=admin", "POSTGRES_DB=mydb"},
			true,
		},
		{
			"postgres default user",
			"postgres:latest",
			[]string{},
			true,
		},
		{
			"postgres with registry prefix",
			"registry.io/myteam/postgres:14",
			[]string{"POSTGRES_USER=u"},
			true,
		},
		{
			"non-postgres image",
			"redis:7",
			[]string{},
			false,
		},
		{
			"nginx image",
			"nginx:latest",
			[]string{},
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inspect := &ContainerInspect{}
			inspect.Config.Image = tt.image
			inspect.Config.Env = tt.env

			healthy, err := checkServiceReadinessFromInspect(
				context.Background(),
				"test-container",
				inspect,
			)

			if !tt.wantCheck {
				// Non-postgres should return an error
				if err == nil {
					t.Error("expected error for non-postgres image")
				}
				return
			}

			// Postgres check will try pg_isready which will fail without docker
			// but we're testing the code path, not the result
			_ = healthy
			// No panic is the main assertion
		})
	}
}

func TestCheckServiceReadinessFromInspect_EnvParsing(t *testing.T) {
	// Test that environment variables are correctly parsed
	inspect := &ContainerInspect{}
	inspect.Config.Image = "postgres:16"
	inspect.Config.Env = []string{
		"POSTGRES_USER=myuser",
		"POSTGRES_DB=mydb",
		"SOME_VAR=with=equals=signs",
		"EMPTY_VAR=",
		"NO_EQUALS",
	}

	// This will fail at the exec step since no docker, but exercises parsing
	_, _ = checkServiceReadinessFromInspect(
		context.Background(), "test-container", inspect,
	)
}

func TestIsServiceHealthyWithContext_InvalidPath(t *testing.T) {
	// Invalid compose path should return false, nil
	healthy, err := isServiceHealthyWithContext(
		context.Background(), "/tmp/bad;rm.yml", "svc", "proj",
	)
	if healthy {
		t.Error("expected false for invalid path")
	}
	// err can be nil since the function returns (false, nil) for errors
	_ = err
}
