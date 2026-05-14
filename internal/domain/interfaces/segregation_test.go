package interfaces_test

import (
	"context"
	"testing"

	"raioz/internal/domain/interfaces"
	"raioz/internal/mocks"
)

// TestSegregatedInterfaceMocking demonstrates that a caller needing
// only ContainerManager can mock just that surface — no need to
// implement the full DockerRunner.
//
// ADR-012 documents the segregation contract: a test that only needs
// ContainerManager should be able to mock just that surface.
func TestSegregatedInterfaceMocking(t *testing.T) {
	// Compile-time proof: MockDockerRunner satisfies every small
	// interface. Test fixtures can pass the same mock down to a
	// function that asks for ContainerManager, ComposeRunner, etc.
	var (
		_ interfaces.ContainerManager = (*mocks.MockDockerRunner)(nil)
		_ interfaces.ComposeRunner    = (*mocks.MockDockerRunner)(nil)
		_ interfaces.NetworkManager   = (*mocks.MockDockerRunner)(nil)
		_ interfaces.VolumeManager    = (*mocks.MockDockerRunner)(nil)
		_ interfaces.ImageValidator   = (*mocks.MockDockerRunner)(nil)
		_ interfaces.PortValidator    = (*mocks.MockDockerRunner)(nil)
	)

	// Functional proof: a function asking only for ContainerManager
	// gets a working mock from a single Func override. The aggregate
	// DockerRunner is never constructed.
	called := false
	probe := func(ctx context.Context, cm interfaces.ContainerManager) (bool, error) {
		return cm.IsProjectActive(ctx, "", "p")
	}

	m := &mocks.MockDockerRunner{
		IsProjectActiveFunc: func(ctx context.Context, ws, p string) (bool, error) {
			called = true
			return true, nil
		},
	}

	got, err := probe(context.Background(), m)
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	if !got {
		t.Error("expected active=true")
	}
	if !called {
		t.Error("expected IsProjectActiveFunc to be invoked")
	}
}
